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
	for w := 1; w < 16; w++ {
		wg.Add(1)
		go worker(jobs, results, &wg)
	}

	accounts := 0
	for it.Next() {
		slimData := it.Account()
		codeHash, err := codeHashFromRLP(slimData)
		if err != nil {
			return err
		}
		if len(codeHash) == 0 {
			continue
		}

		code, err := codeGetter.ContractCode(common.BytesToHash(codeHash))
		if err != nil {
			return err
		}

		jobs <- code
		accounts++
	}
	close(jobs)

	for r := range results {
		log.Info("CodeRoot: %v\n", r)
	}

	wg.Wait()
	log.Info("Merkleized code", "accounts", accounts, "elapsed", time.Since(start))

	return nil
}

func worker(jobs <-chan []byte, results chan<- common.Hash, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
		root, err := MerkleizeInMemory(j, 32)
		if err != nil {
			log.Warn("Error in merkleizing code: %v\n", err)
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
