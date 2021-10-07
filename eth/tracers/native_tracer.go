package tracers

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

type NativeTracer struct {
	tracer PluginAPI
}

func NewNativeTracer(tracer PluginAPI) (*NativeTracer, error) {
	return &NativeTracer{tracer: tracer}, nil
}

func (t *NativeTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	log.Info("NativeTracer.CaptureStart")
}

func (t *NativeTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, rData []byte, depth int, err error) {
	t.tracer.Step(op)
}

func (t *NativeTracer) CaptureFault(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
}

func (t *NativeTracer) CaptureEnd(output []byte, gasUsed uint64, t_ time.Duration, err error) {
}

func (t *NativeTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
}

func (t *NativeTracer) CaptureExit(output []byte, gasUsed uint64, err error) {}

func (t *NativeTracer) GetResult() (json.RawMessage, error) {
	return t.tracer.Result()
}
