package tracers

import (
	"encoding/json"
	"errors"
	"math/big"
	"plugin"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

type StepFunc func(op vm.OpCode)
type ResultFunc func() (json.RawMessage, error)

type PluginTracer struct {
	plugin *plugin.Plugin
	step   StepFunc
	result ResultFunc
}

func NewPluginTracer(path string) (*PluginTracer, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}
	stepSym, err := p.Lookup("Step")
	if err != nil {
		return nil, err
	}
	step, ok := stepSym.(StepFunc)
	if !ok {
		return nil, errors.New("plugin has invalid step signature")
	}
	resultSym, err := p.Lookup("Result")
	if err != nil {
		return nil, err
	}
	result, ok := resultSym.(ResultFunc)
	if !ok {
		return nil, errors.New("plugin has invalid result signature")
	}
	return &PluginTracer{plugin: p, step: step, result: result}, nil
}

func (t *PluginTracer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) error {
	log.Info("PluginTracer.CaptureStart")
	return nil
}

func (t *PluginTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, memory *vm.Memory, stack *vm.Stack, rStack *vm.ReturnStack, rData []byte, contract *vm.Contract, depth int, err error) error {
	t.step(op)
	return nil
}

func (t *PluginTracer) CaptureFault(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, memory *vm.Memory, stack *vm.Stack, rStack *vm.ReturnStack, contract *vm.Contract, depth int, err error) error {
	return nil
}

func (t *PluginTracer) CaptureEnd(output []byte, gasUsed uint64, t_ time.Duration, err error) error {
	return nil
}

func (t *PluginTracer) GetResult() (json.RawMessage, error) {
	return t.result()
}
