package tracers

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type UnigramTracer struct {
	hist map[OpCode]int
}

func NewUnigram() *UnigramTracer {
	return &UnigramTracer{hist: make(map[OpCode]int)}
}

// CaptureStart implements the Tracer interface to initialize the tracing operation.
func (t *UnigramTracer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) error {
}

func (t *UnigramTracer) CaptureState(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, stack *Stack, rStack *ReturnStack, rData []byte, contract *Contract, depth int, err error) error {
	if _, ok := t.hist[op]; !ok {
		t.hist[op] = 1
	} else {
		t.hist[op]++
	}
}

func (t *UnigramTracer) CaptureFault(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, stack *Stack, rStack *ReturnStack, contract *Contract, depth int, err error) error {
}

func (t *UnigramTracer) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) error {
}

func (t *UnigramTracer) GetResult() ([]byte, error) {
	return json.Marshal(t.hist)
}
