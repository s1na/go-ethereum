package tracers

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type NativeTracer struct {
}

func NewNative() *NativeTracer {
	return &NativeTracer{}
}

// CaptureStart implements the Tracer interface to initialize the tracing operation.
func (t *NativeTracer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) error {
}

func (t *NativeTracer) CaptureState(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, stack *Stack, rStack *ReturnStack, rData []byte, contract *Contract, depth int, err error) error {
}

func (t *NativeTracer) CaptureFault(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, stack *Stack, rStack *ReturnStack, contract *Contract, depth int, err error) error {
}

func (t *NativeTracer) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) error {
}
