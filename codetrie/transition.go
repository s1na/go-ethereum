package codetrie

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

type CodeGetter interface {
	ContractCode(common.Hash) ([]byte, error)
}

func Transition(codeGetter CodeGetter, it snapshot.AccountIterator) error {
	start := time.Now()
	accounts := 0
	duplicates := 0

	index := make(map[string]common.Hash)

	for it.Next() {
		slimData := it.Account()
		codeHash, err := codeHashFromRLP(slimData)
		if err != nil {
			return err
		}
		if len(codeHash) == 0 {
			continue
		}
		codeHashStr := string(codeHash)
		if _, exists := index[codeHashStr]; exists {
			duplicates++
			continue
		}

		code, err := codeGetter.ContractCode(common.BytesToHash(codeHash))
		if err != nil {
			return err
		}

		root, err := MerkleizeInMemory(code, 32)
		index[codeHashStr] = root
		accounts++
	}

	log.Info("Merkleized code", "accounts", accounts, "duplicates", duplicates, "elapsed", time.Since(start))
	return nil
}

// Merkleizes multiple contracts concurrently
func TransitionConcurrent(codeGetter CodeGetter, it snapshot.AccountIterator) error {
	var wg sync.WaitGroup

	start := time.Now()

	jobs := make(chan common.Hash)
	results := make(chan common.Hash)
	errCh := make(chan error)
	waitCh := make(chan struct{})

	var index sync.Map

	numWorkers := 16
	for w := 1; w < numWorkers; w++ {
		wg.Add(1)
		go worker(codeGetter, &index, jobs, results, errCh, &wg)
	}

	accounts := 0
	go func() {
		for it.Next() {
			slimData := it.Account()
			codeHash, err := codeHashFromRLP(slimData)
			if err != nil {
				errCh <- err
				break
			}
			if len(codeHash) == 0 {
				continue
			}

			jobs <- common.BytesToHash(codeHash)
			accounts++
		}
		close(jobs)
	}()

	// Wait for workers to be done and
	// send a signal by closing a channel.
	// This is being done as a work-around for
	// using WG in the select statement below.
	go func() {
		wg.Wait()
		close(waitCh)
	}()

ResultLoop:
	for {
		select {
		case <-results:
			// TODO: process resulting codeRoot
			//log.Info("Received merkleization result", "root", r)
		case err := <-errCh:
			log.Warn("Merkleization task failed", "error", err)
			break ResultLoop
		case <-waitCh:
			break ResultLoop
		}
	}
	/*for r := range results {
		log.Info("CodeRoot: %v\n", r)
	}*/
	close(results)

	log.Info("Merkleized code", "accounts", accounts, "elapsed", time.Since(start))

	return nil
}

func worker(codeGetter CodeGetter, index *sync.Map, jobs <-chan common.Hash, results chan<- common.Hash, errCh chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
		codeHashStr := string(j.Bytes())
		// Short circuit if code has been merkleized
		r, inIndex := index.Load(codeHashStr)
		if inIndex {
			results <- r.(common.Hash)
			continue
		}

		code, err := codeGetter.ContractCode(j)
		if err != nil {
			errCh <- err
			break
		}

		root, err := MerkleizeInMemory(code, 32)
		if err != nil {
			errCh <- err
			break
		} else {
			if !inIndex {
				index.Store(codeHashStr, root)
			}
			results <- root
		}
	}
}

func codeHashFromRLP(data []byte) ([]byte, error) {
	var account snapshot.Account
	if err := rlp.DecodeBytes(data, &account); err != nil {
		return []byte{}, err
	}
	return account.CodeHash, nil
}

func BenchMerkleizationOverhead(codeGetter CodeGetter, it snapshot.AccountIterator) error {
	start := time.Now()
	accounts := 0
	duplicates := 0

	index := make(map[string]common.Hash)

	type ContractStat struct {
		CodeLen  int
		Code     string
		Duration int64 // overhead in ns
		overhead time.Duration
	}
	stats := make([]ContractStat, 0, 0)

	for it.Next() {
		slimData := it.Account()
		codeHash, err := codeHashFromRLP(slimData)
		if err != nil {
			return err
		}
		if len(codeHash) == 0 {
			continue
		}
		codeHashStr := string(codeHash)
		if _, exists := index[codeHashStr]; exists {
			duplicates++
			continue
		}

		code, err := codeGetter.ContractCode(common.BytesToHash(codeHash))
		if err != nil {
			return err
		}

		benchStart := time.Now()
		root, err := MerkleizeStack(code, 32)
		overhead := time.Since(benchStart)
		stats = append(stats, ContractStat{CodeLen: len(code), Code: hex.EncodeToString(code), Duration: overhead.Nanoseconds(), overhead: overhead})
		index[codeHashStr] = root
		accounts++
	}

	log.Info("Merkleized code", "accounts", accounts, "duplicates", duplicates, "elapsed", time.Since(start))
	/*cw := csv.NewWriter(os.Stdout)
	for _, item := range data {
		if err := cw.Write([]string{strconv.Itoa(item.codeLen), strconv.FormatInt(item.overhead.Nanoseconds(), 10)}); err != nil {
			log.Warn("error csv", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Warn("after csv error", err)
	}*/

	// Write contract codes to json file
	type Schema struct {
		Stats []ContractStat
	}
	sort.Slice(stats, func(a, b int) bool {
		return stats[a].Duration < stats[b].Duration
	})

	data := Schema{Stats: stats}
	jdata, err := json.Marshal(data)
	if err != nil {
		log.Warn("err encoding json", err)
	}
	ioutil.WriteFile("contracts.json", jdata, 0644)

	return nil
}
