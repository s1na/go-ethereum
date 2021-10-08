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
    t.ctx = Context{
        From: from,
        To: to,
        Input: common.CopyBytes(input),
        Gas: gas,
        GasPrice: env.TxContext.GasPrice,
        Value: value,
		Type: "CALL",
        activePrecompiles: vm.ActivePrecompiles(rules)
    }
    if create {
        t.ctx.Type = "CREATE"
    }
    t.activePrecompiles = vm.ActivePrecompiles(rules)

    // Compute intrinsic gas                                                                         
    isHomestead := env.ChainConfig().IsHomestead(env.Context.BlockNumber)
    isIstanbul := env.ChainConfig().IsIstanbul(env.Context.BlockNumber)
    intrinsicGas, err := core.IntrinsicGas(input, nil, jst.ctx["type"] == "CREATE", isHomestead, isIstanbul)
    if err != nil {
        // TODO why failure is silent here?
        return
    }

    t.ctxt.IntrinsictGas = intrinsicGas
}

func (t *NativeTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, rData []byte, depth int, err error) {
    if !t.traceSteps {
        return
    }
    if t.err != nil {
        return
    }
    // If tracing was interrupted, set the error and stop
    if atomic.LoadUint32(&t.interrupt) > 0 {
        t.err = t.reason
        env.Cancel()
        return
    }

    memory := PluginMemoryWrapper{scope.Memory}
    contract := PluginContractWrapper{scope.Contract}
    stack := PluginStackWrapper{scope.Stack}

    log := StepLog{
        Op: op,
        PC: uint(pc),
        Gas: uint(gas),
        Cost: uint(cost),
        Depth: uint(depth),
        Refund: uint(env.StateDB.GetRefund()),
        Error: err,
    }
	t.tracer.Step(t.ctx, t.log, env)
}

func (t *NativeTracer) CaptureFault(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
	if t.Error != nil {
		return
	}
    t.Error = err
}

func (t *NativeTracer) CaptureEnd(output []byte, gasUsed uint64, t_ time.Duration, err error) {
	t.ctx.Output = common.CopyBytes(output)
	t.ctx.GasUsed = gasUsed
	if err != nil {
		t.ctx.Error = errors.New(err.Error())
	}
}

func (t *NativeTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	if !t.traceCallFrames {
		return
	}
    if t.err != nil {
        return
    }
    // If tracing was interrupted, set the error and stop 
    if atomic.LoadUint32(&t.interrupt) > 0 {
        t.err = t.reason
        return
    }

	input = common.CopyBytes(input)
	if value != nil {
		value = common.CopyBig(value)
	}
	t.tracer.Enter(typ, from, to, input, gas, value)
}

func (t *NativeTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
    if !t.traceCallFrames {
        return
    }
    // If tracing was interrupted, set the error and stop
    if atomic.LoadUint32(&t.interrupt) > 0 {
        t.err = t.reason
        return
    }
	output = common.CopyBytes(output)
	t.tracer.Exit(output, gasUsed, err)
}

func (t *NativeTracer) GetResult() (json.RawMessage, error) {
	return t.tracer.Result(t.ctx)
}

func (t *NativeTracer) Stop(err error) {
    t.reason = err
    atomic.StoreUint32(&t.interrupt, 1)
}
