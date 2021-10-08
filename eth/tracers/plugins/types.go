package plugins

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// transaction context
type PluginContext struct {
	Type    string
	From    common.Address
	To      common.Address
	Input   []byte
	Gas     uint64
	Value   *big.Int
	Output  []byte
	GasUsed uint64
	Error   error
}
