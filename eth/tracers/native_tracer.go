package tracers

import (
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/plugins"
	"github.com/ethereum/go-ethereum/log"
)

type NativeTracer struct {
	tracer PluginAPI
	ctx    *plugins.PluginContext
}

func NewNativeTracer(tracer PluginAPI) (*NativeTracer, error) {
	return &NativeTracer{tracer: tracer, ctx: new(plugins.PluginContext)}, nil
}

func (t *NativeTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	log.Info("NativeTracer.CaptureStart")
	t.ctx.Type = "CALL"
	if create {
		t.ctx.Type = "CREATE"
	}
	t.ctx.From = from
	t.ctx.To = to
	t.ctx.Input = common.CopyBytes(input)
	t.ctx.Gas = gas
	t.ctx.Value = common.CopyBig(value)
}

func (t *NativeTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, rData []byte, depth int, err error) {
	t.tracer.Step(op)
}

func (t *NativeTracer) CaptureFault(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
}

func (t *NativeTracer) CaptureEnd(output []byte, gasUsed uint64, t_ time.Duration, err error) {
	t.ctx.Output = common.CopyBytes(output)
	t.ctx.GasUsed = gasUsed
	if err != nil {
		t.ctx.Error = errors.New(err.Error())
	}
}

func (t *NativeTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	input = common.CopyBytes(input)
	if value != nil {
		value = common.CopyBig(value)
	}
	t.tracer.Enter(typ, from, to, input, gas, value)
}

func (t *NativeTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
	output = common.CopyBytes(output)
	t.tracer.Exit(output, gasUsed, err)
}

func (t *NativeTracer) GetResult() (json.RawMessage, error) {
	return t.tracer.Result(t.ctx)
}
