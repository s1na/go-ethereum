package tracers

import (
	"encoding/json"
	"errors"
	"math/big"
	"time"
    "sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/plugins"
	"github.com/ethereum/go-ethereum/log"
)

type NativeTracer struct {
	tracer PluginAPI
	ctx    *plugins.PluginContext
    interrupt *uint32
    traceSteps bool
    err error
    reason error
    traceCallFrames bool
}

func NewNativeTracer(tracer PluginAPI) (*NativeTracer, error) {
    interrupt := new(uint32)
	return &NativeTracer{tracer: tracer, ctx: new(plugins.PluginContext), interrupt: interrupt}, nil
}

func (t *NativeTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	log.Info("NativeTracer.CaptureStart")
    rules := env.ChainConfig().Rules(env.Context.BlockNumber)
    t.ctx = &plugins.PluginContext{
        From: from,
        To: to,
        Input: common.CopyBytes(input),
        Gas: gas,
        GasPrice: env.TxContext.GasPrice,
        Value: value,
		Type: "CALL",
        ActivePrecompiles: vm.ActivePrecompiles(rules),
    }
    if create {
        t.ctx.Type = "CREATE"
    }

    // Compute intrinsic gas                                                                         
    isHomestead := env.ChainConfig().IsHomestead(env.Context.BlockNumber)
    isIstanbul := env.ChainConfig().IsIstanbul(env.Context.BlockNumber)
    intrinsicGas, err := core.IntrinsicGas(input, nil, t.ctx.Type == "CREATE", isHomestead, isIstanbul)
    if err != nil {
        // TODO why failure is silent here?
        return
    }

    t.ctx.IntrinsicGas = intrinsicGas
    t.tracer.Start(t.ctx, env.StateDB)
}

func (t *NativeTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
    if !t.traceSteps {
        return
    }
    if t.err != nil {
        return
    }
    // If tracing was interrupted, set the error and stop
    if atomic.LoadUint32(t.interrupt) > 0 {
        t.err = t.reason
        env.Cancel()
        return
    }

    memory := plugins.MemoryWrapper{scope.Memory}
    contract := plugins.ContractWrapper{scope.Contract}
    stack := plugins.StackWrapper{scope.Stack}

    log := plugins.StepLog{
        Op: op,
        Pc: uint(pc),
        Gas: uint(gas),
        Cost: uint(cost),
        Depth: uint(depth),
        Refund: uint(env.StateDB.GetRefund()),
        Memory: memory,
        Contract: contract,
        Stack: stack,
    }
	t.tracer.Step(&log)
}

func (t *NativeTracer) CaptureFault(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
	if t.err != nil {
		return
	}
    t.err = err
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
    if atomic.LoadUint32(t.interrupt) > 0 {
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
    if atomic.LoadUint32(t.interrupt) > 0 {
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
    atomic.StoreUint32(t.interrupt, 1)
}
