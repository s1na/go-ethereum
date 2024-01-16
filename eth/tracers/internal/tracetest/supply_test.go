// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package tracetest

import (
	"bufio"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tracers/directory"
	"github.com/ethereum/go-ethereum/eth/tracers/live"
	"github.com/ethereum/go-ethereum/params"

	// Force-load live packages, to trigger registration
	_ "github.com/ethereum/go-ethereum/eth/tracers/live"
)

func TestSupplyTracer(t *testing.T) {
	var (
		merger = consensus.NewMerger(rawdb.NewMemoryDatabase())
		engine = beacon.New(ethash.NewFaker())

		aa = common.HexToAddress("0x000000000000000000000000000000000000aaaa")
		// A sender who makes transactions, has some funds
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		funds   = new(big.Int).Mul(common.Big1, big.NewInt(params.Ether))
		config  = *params.AllEthashProtocolChanges

		gspec = &core.Genesis{
			Config: &config,
			Alloc: core.GenesisAlloc{
				addr1: {Balance: funds},
				addr2: {Balance: funds},
				// The address 0xAAAA sloads 0x00 and 0x01
				aa: {
					Code: []byte{
						byte(vm.PC),
						byte(vm.PC),
						byte(vm.SLOAD),
						byte(vm.SLOAD),
					},
					Nonce:   0,
					Balance: big.NewInt(0),
				},
			},
		}
	)

	gspec.Config.BerlinBlock = big.NewInt(1)
	gspec.Config.LondonBlock = big.NewInt(2)
	gspec.Config.ArrowGlacierBlock = big.NewInt(2)
	gspec.Config.GrayGlacierBlock = big.NewInt(2)

	blockTime := uint64(10)
	shanghaiBlockNumber := uint64(3)
	shanghaiTime := gspec.Timestamp + (blockTime * shanghaiBlockNumber) + 1
	gspec.Config.ShanghaiTime = &shanghaiTime

	signer := types.LatestSigner(gspec.Config)

	tests := []struct {
		name     string
		input    func(b *core.BlockGen)
		expected live.SupplyInfo
	}{
		{"Genesis",
			func(b *core.BlockGen) {},
			live.SupplyInfo{
				Delta:       new(big.Int).Mul(common.Big2, big.NewInt(params.Ether)),
				Reward:      common.Big0,
				Withdrawals: common.Big0,
				Burn:        common.Big0,
				Number:      0,
				Hash:        common.HexToHash("0xa20143bad7de540bd4747be05f9a17d2708aff19c8db72fa9026cc11bc151727"), ParentHash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
			},
		},
		{"Rewards",
			func(b *core.BlockGen) {},
			live.SupplyInfo{
				Delta:       new(big.Int).Mul(common.Big2, big.NewInt(params.Ether)),
				Reward:      new(big.Int).Mul(common.Big2, big.NewInt(params.Ether)),
				Withdrawals: common.Big0,
				Burn:        common.Big0,
				Number:      1,
				Hash:        common.HexToHash("0x3053472d528da389264ca290e61d330fec1138ccb9d9ef686d5f1a8b68069b3b"), ParentHash: common.HexToHash("0xa20143bad7de540bd4747be05f9a17d2708aff19c8db72fa9026cc11bc151727"),
			},
		},
		{"London",
			func(b *core.BlockGen) {
				// One transaction to 0xAAAA
				accesses := types.AccessList{types.AccessTuple{
					Address:     aa,
					StorageKeys: []common.Hash{{0}},
				}}

				txdata := &types.DynamicFeeTx{
					ChainID:    gspec.Config.ChainID,
					Nonce:      0,
					To:         &aa,
					Gas:        30000,
					GasFeeCap:  new(big.Int).Mul(big.NewInt(5), big.NewInt(params.GWei)),
					GasTipCap:  big.NewInt(2),
					AccessList: accesses,
					Data:       []byte{},
				}
				tx := types.NewTx(txdata)
				tx, _ = types.SignTx(tx, signer, key1)

				b.AddTx(tx)
			},
			live.SupplyInfo{
				Delta:       big.NewInt(1999972496000000000),
				Reward:      new(big.Int).Mul(common.Big2, big.NewInt(params.Ether)),
				Withdrawals: common.Big0,
				Burn:        big.NewInt(27504000000000),
				Number:      2,
				Hash:        common.HexToHash("0x2bb5afb410af25822a61b281393cf19d89663554422534d317de8f003b29e59c"), ParentHash: common.HexToHash("0x3053472d528da389264ca290e61d330fec1138ccb9d9ef686d5f1a8b68069b3b"),
			},
		},
		{"Merge",
			func(b *core.BlockGen) {},
			live.SupplyInfo{
				Delta:       common.Big0,
				Reward:      common.Big0,
				Withdrawals: common.Big0,
				Burn:        common.Big0,
				Number:      3,
				Hash:        common.HexToHash("0x07d9204c8de4fc288e75ba8d90f4418314460492f8408d82896df2fd82d946e0"), ParentHash: common.HexToHash("0x2bb5afb410af25822a61b281393cf19d89663554422534d317de8f003b29e59c"),
			},
		},
		{"Withdrawals",
			func(b *core.BlockGen) {
				b.AddWithdrawal(&types.Withdrawal{
					Validator: 42,
					Address:   common.Address{0xee},
					Amount:    1337,
				})
			},
			live.SupplyInfo{
				Delta:       big.NewInt(1337000000000),
				Reward:      common.Big0,
				Withdrawals: big.NewInt(1337000000000),
				Burn:        common.Big0,
				Number:      4,
				Hash:        common.HexToHash("0x9259ce2600b670f7369f699bb325cc57f646d0f392c98b29022596f54252db95"), ParentHash: common.HexToHash("0x07d9204c8de4fc288e75ba8d90f4418314460492f8408d82896df2fd82d946e0"),
			},
		},
	}

	// Load supply tracer
	tracer, err := directory.LiveDirectory.New("supply")
	if err != nil {
		t.Fatalf("failed to create call tracer: %v", err)
	}

	chain, err := core.NewBlockChain(rawdb.NewMemoryDatabase(), core.DefaultCacheConfigWithScheme(rawdb.PathScheme), gspec, nil, engine, vm.Config{Tracer: tracer}, nil, nil)
	if err != nil {
		t.Fatalf("failed to create tester chain: %v", err)
	}
	defer chain.Stop()

	genDb, blocks, _ := core.GenerateChainWithGenesis(gspec, engine, 1, func(i int, b *core.BlockGen) {
		b.SetCoinbase(common.Address{1})
		tests[b.Number().Uint64()].input(b)
	})

	if n, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("block %d: failed to insert into chain: %v", n, err)
	}

	// London hard fork
	gspec.BaseFee = big.NewInt(params.InitialBaseFee)

	parent := blocks[len(blocks)-1]
	blocks, _ = core.GenerateChain(gspec.Config, parent, engine, genDb, 1, func(i int, b *core.BlockGen) {
		b.SetCoinbase(common.Address{2})
		tests[b.Number().Uint64()].input(b)
	})

	if n, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("block %d: failed to insert into chain: %v", n, err)
	}

	// The Merge
	merger.ReachTTD()
	merger.FinalizePoS()
	// Set the terminal total difficulty in the config
	ttd := big.NewInt(int64(len(blocks)))
	ttd.Mul(ttd, params.GenesisDifficulty)
	gspec.Config.TerminalTotalDifficulty = ttd

	parent = blocks[len(blocks)-1]
	blocks, _ = core.GenerateChain(gspec.Config, parent, engine, genDb, 2, func(i int, b *core.BlockGen) {
		b.SetPoS()
		b.SetCoinbase(common.Address{3})
		tests[b.Number().Uint64()].input(b)
	})

	if n, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("block %d: failed to insert into chain: %v", n, err)
	}

	// Check and compare the results
	// TODO: replace file to pass results
	file, err := os.OpenFile("supply.txt", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	testedBlockNumber := 0

	for scanner.Scan() {
		blockBytes := scanner.Bytes()

		var actual live.SupplyInfo
		if err := json.Unmarshal(blockBytes, &actual); err != nil {
			t.Fatalf("failed to unmarshal result for block %d: %v", testedBlockNumber, err)
		}

		test := tests[testedBlockNumber]
		expected := test.expected
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("Test \"%v\" at block %d: incorrect supply info: expected %+v, got %+v", test.name, testedBlockNumber, expected, actual)
		}

		testedBlockNumber++
	}
}
