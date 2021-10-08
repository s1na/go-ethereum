package plugins

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

type ContractWrapper struct {
    Contract *vm.Contract
}

type StackWrapper struct {
	Stack *vm.Stack
}

func (sw *StackWrapper) Peek(idx int) *big.Int {
    if len(sw.Stack.Data()) <= idx || idx < 0 {
        log.Warn("Tracer accessed out of bound stack", "size", len(sw.Stack.Data()), "index", idx)
        return new(big.Int)
    }
    return sw.Stack.Back(idx).ToBig()
}

type MemoryWrapper struct {
	Memory *vm.Memory
}

func (mw *MemoryWrapper) Slice(begin, end int64) []byte {
    if end == begin {
        return []byte{}
    }
    if end < begin || begin < 0 {
        log.Warn("Tracer accessed out of bound memory", "offset", begin, "end", end)
        return nil
    }
    if mw.Memory.Len() < int(end) {
        log.Warn("Tracer accessed out of bound memory", "available", mw.Memory.Len(), "offset", begin, "size", end-begin)
        return nil
    }
    return mw.Memory.GetCopy(begin, end-begin)
}

func (mw *MemoryWrapper) GetUint(addr int64) *big.Int {
    if mw.Memory.Len() < int(addr)+32 || addr < 0 {
        log.Warn("Tracer accessed out of bound memory", "available", mw.Memory.Len(), "offset", addr, "size", 32)
        return new(big.Int)
    }
    return new(big.Int).SetBytes(mw.Memory.GetPtr(addr, 32))
}

type StepLog struct {
	Pc       uint
	Gas      uint
	Cost     uint
	Depth    uint
	Refund   uint
	Memory   MemoryWrapper
	Contract ContractWrapper
	Stack    StackWrapper
    Op vm.OpCode
}

// TODO make this an interface exposed to the plugin, make some fields private and move out of this package.
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
    GasPrice *big.Int
	Error   error
    ActivePrecompiles []common.Address
    IntrinsicGas uint64
}
