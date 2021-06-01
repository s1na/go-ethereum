package main

import (
	"bytes"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/verkle"
	verkleLib "github.com/gballet/go-verkle"
	cli "gopkg.in/urfave/cli.v1"
)

var (
	verkleCommand = cli.Command{
		Name:        "verkle",
		Usage:       "A set of commands based for operating verkle trees",
		Category:    "MISCELLANEOUS COMMANDS",
		Description: "",
		Subcommands: []cli.Command{
			{
				Name:      "compute-commitments",
				Usage:     "Traverse the state and compute the root commitment of the state",
				ArgsUsage: "<root>",
				Action:    utils.MigrateFlags(computeCommitment),
				Category:  "MISCELLANEOUS COMMANDS",
				Flags: []cli.Flag{
					utils.DataDirFlag,
					utils.RopstenFlag,
					utils.RinkebyFlag,
					utils.GoerliFlag,
					utils.LegacyTestnetFlag,
				},
				Description: `
geth snapshot traverse-state <state-root>
will traverse the whole state from the given state root and will abort if any
referenced trie node or contract code is missing. This command can be used for
state integrity verification. The default checking target is the HEAD state.

It's also usable without snapshot enabled.
`,
			},
			{
				Name:      "convert-snapshot",
				Usage:     "Produce a verkle-compatible snapshot",
				ArgsUsage: "<root>",
				Action:    utils.MigrateFlags(convertSnapshot),
				Category:  "MISCELLANEOUS COMMANDS",
				Flags: []cli.Flag{
					utils.DataDirFlag,
					utils.RopstenFlag,
					utils.RinkebyFlag,
					utils.GoerliFlag,
					utils.LegacyTestnetFlag,
				},
				Description: ``,
			},
		},
	}
)

func computeCommitment(ctx *cli.Context) error {
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	chain, chaindb := utils.MakeChain(ctx, stack, true)
	defer chaindb.Close()

	verkledb, err := stack.OpenDatabase("verkle", 0, 0, "")
	if err != nil {
		log.Error("Failed to open db for verkle nodes", "error", err)
		return err
	}
	defer verkledb.Close()

	nodesCh := make(chan verkleLib.FlushableNode)
	verkleGenerate := func(db ethdb.KeyValueWriter, in chan snapshot.TrieKV, out chan common.Hash) {
		t := verkleLib.New(10)
		for leaf := range in {
			t.InsertOrdered(common.CopyBytes(leaf.Key[:]), leaf.Value, nodesCh)
		}
		// Flush remaining nodes to nodes channel
		rootNode, ok := t.(*verkleLib.InternalNode)
		if !ok {
			panic("verkle tree has invalid root node")
		}
		root := t.Hash()
		rootNode.Flush(nodesCh)
		out <- root
	}

	nodesCount := 0
	go func() {
		for fn := range nodesCh {
			nodesCount++
			value, err := fn.Node.Serialize()
			if err != nil {
				log.Error("Failed to serialize verkle node", "error", err)
			}
			if err := verkledb.Put(fn.Hash[:], value); err != nil {
				log.Error("Failed to write verkle node to db", "error", err)
			}
		}
	}()

	if ctx.NArg() > 1 {
		log.Error("Too many arguments given")
		return errors.New("too many arguments")
	}
	// Use the HEAD root as the default
	head := chain.CurrentBlock()
	if head == nil {
		log.Error("Head block is missing")
		return errors.New("head block is missing")
	}
	var root common.Hash
	if ctx.NArg() == 1 {
		root, err = parseRoot(ctx.Args()[0])
		if err != nil {
			log.Error("Failed to resolve state root", "error", err)
			return err
		}
		log.Info("Start traversing the state", "root", root)
	} else {
		root = head.Root()
		log.Info("Start traversing the state", "root", root, "number", head.NumberU64())
	}

	triedb := trie.NewDatabase(chaindb)
	t, err := snapshot.New(chaindb, triedb, 256, chain.CurrentBlock().Root(), false, false, false)
	if err != nil {
		log.Error("Failed to open snapshot tree", "error", err)
		return err
	}

	if err := t.ComputeVerkleCommitment(root, verkleGenerate); err != nil {
		log.Error("Failed to compute verkle commitment", "error", err)
	}
	log.Info("Number of nodes written to DB\n", "nodes", nodesCount)
	return nil
}

func convertSnapshot(ctx *cli.Context) error {
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	chain, chaindb := utils.MakeChain(ctx, stack, true)
	defer chaindb.Close()

	verkledb, err := stack.OpenDatabase("verkle", 0, 0, "")
	if err != nil {
		log.Error("Failed to open db for verkle nodes", "error", err)
		return err
	}
	defer verkledb.Close()

	if ctx.NArg() > 1 {
		log.Error("Too many arguments given")
		return errors.New("too many arguments")
	}
	// Use the HEAD root as the default
	head := chain.CurrentBlock()
	if head == nil {
		log.Error("Head block is missing")
		return errors.New("head block is missing")
	}
	root := head.Root()
	log.Info("Start traversing the state", "root", root, "number", head.NumberU64())

	triedb := trie.NewDatabase(chaindb)
	t, err := snapshot.New(chaindb, triedb, 256, chain.CurrentBlock().Root(), false, false, false)
	if err != nil {
		log.Error("Failed to open snapshot tree", "error", err)
		return err
	}

	var (
		accounts = 0
		slots    = 0
		start    = time.Now()
	)

	acctIt, err := t.AccountIterator(root, common.Hash{})
	if err != nil {
		return err
	}
	defer acctIt.Release()

	for acctIt.Next() {
		key := acctIt.Hash()
		accountData := acctIt.Account()
		verkle.WriteAccountSnapshot(verkledb, key, accountData)
		accounts++

		account, err := snapshot.FullAccount(accountData)
		if err != nil {
			log.Error("Failed to read account from snapshot", "error", err)
			return err
		}
		if !bytes.Equal(account.Root, emptyRoot[:]) {
			storageIt, err := t.StorageIterator(root, key, common.Hash{})
			if err != nil {
				log.Error("Failed to initiate storage iterator", "error", err)
				return err
			}
			defer storageIt.Release()

			for storageIt.Next() {
				skey := storageIt.Hash()
				slot := storageIt.Slot()
				verkle.WriteStorageSnapshot(verkledb, key, skey, slot)
				slots++
			}
		}
	}

	log.Info("Converted snapshot to a verkle-compatible one", "accounts", accounts, "slots", slots, "time", time.Since(start))

	return nil
}
