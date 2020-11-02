package codetrie

import (
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

	numWorkers := 16
	for w := 1; w < numWorkers; w++ {
		wg.Add(1)
		go worker(codeGetter, jobs, results, errCh, &wg)
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
		case r := <-results:
			log.Info("Received merkleization result", "root", r)
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

func worker(codeGetter CodeGetter, jobs <-chan common.Hash, results chan<- common.Hash, errCh chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
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
