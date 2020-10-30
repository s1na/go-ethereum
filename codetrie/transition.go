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

// Transition procedure for merkleizing all contract code
func Transition(codeGetter CodeGetter, it snapshot.AccountIterator) error {
	var wg sync.WaitGroup

	start := time.Now()

	jobs := make(chan []byte)
	results := make(chan common.Hash)
	done := make(chan bool)
	errCh := make(chan error)

	numWorkers := 16
	for w := 1; w < numWorkers; w++ {
		wg.Add(1)
		go worker(jobs, results, done, &wg)
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

			code, err := codeGetter.ContractCode(common.BytesToHash(codeHash))
			if err != nil {
				errCh <- err
				break
			}

			jobs <- code
			accounts++
		}
		close(jobs)
	}()

	doneWorkers := 0
	over := false
	for {
		select {
		case r := <-results:
			log.Info("CodeRoot: %v\n", r)
		case err := <-errCh:
			log.Warn("Error: %v\n", err)
			over = true
			break
		case <-done:
			doneWorkers++
			if doneWorkers == numWorkers {
				over = true
				break
			}
		}
		if over {
			break
		}
	}
	/*for r := range results {
		log.Info("CodeRoot: %v\n", r)
	}*/
	close(results)

	wg.Wait()
	log.Info("Merkleized code", "accounts", accounts, "elapsed", time.Since(start))

	return nil
}

func worker(jobs <-chan []byte, results chan<- common.Hash, done chan<- bool, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
		root, err := MerkleizeInMemory(j, 32)
		if err != nil {
			log.Warn("Error in merkleizing code: %v\n", err)
		} else {
			results <- root
		}
	}

	done <- true
}

func codeHashFromRLP(data []byte) ([]byte, error) {
	var account snapshot.Account
	if err := rlp.DecodeBytes(data, &account); err != nil {
		return []byte{}, err
	}
	return account.CodeHash, nil
}
