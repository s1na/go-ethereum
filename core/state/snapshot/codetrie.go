package snapshot

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"time"
)

type CodeGetter interface {
	ContractCode(common.Hash) ([]byte, error)
}

// Transition procedure for merkleizing all contract code
func MerkleizeCode(codeGetter CodeGetter, it AccountIterator) error {
	start := time.Now()
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
		fmt.Printf("Code: %v\n", code)
		accounts++
		break
	}

	log.Info("Merkleized code", "accounts", accounts, "elapsed", time.Since(start))

	return nil
}

func codeHashFromRLP(data []byte) ([]byte, error) {
	var account Account
	if err := rlp.DecodeBytes(data, &account); err != nil {
		return []byte{}, err
	}
	return account.CodeHash, nil
}
