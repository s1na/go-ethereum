package native

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type UnigramTracer struct {
	hist map[vm.OpCode]int
}

func NewUnigram() *UnigramTracer {
	return &UnigramTracer{hist: make(map[vm.OpCode]int)}
}

// CaptureStart implements the Tracer interface to initialize the tracing operation.
func (t *UnigramTracer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) error {
	return nil
}

func (t *UnigramTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, memory *vm.Memory, stack *vm.Stack, rStack *vm.ReturnStack, rData []byte, contract *vm.Contract, depth int, err error) error {
	if _, ok := t.hist[op]; !ok {
		t.hist[op] = 1
	} else {
		t.hist[op]++
	}
	return nil
}

func (t *UnigramTracer) CaptureFault(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, memory *vm.Memory, stack *vm.Stack, rStack *vm.ReturnStack, contract *vm.Contract, depth int, err error) error {
	return nil
}

func (t *UnigramTracer) CaptureEnd(output []byte, gasUsed uint64, t_ time.Duration, err error) error {
	return nil
}

func (t *UnigramTracer) GetResult() ([]byte, error) {
	return json.Marshal(t.hist)
}
