package snapshot

import (
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"time"
)

func MerkleizeCode(it AccountIterator) error {
	start := time.Now()
	accounts := 0
	for it.Next() {
		slimData := it.Account()
		account, err := FullAccount(slimData)
		if err != nil {
			return err
		}
		fmt.Printf("Account: %v\n", account)
		accounts++
		break
	}

	log.Info("Merkleized code", "accounts", accounts, "elapsed", time.Since(start))

	return nil
}
