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

type NewFunc = func() Plugin

type Plugin interface {
	Step(op vm.OpCode)
	Result() (json.RawMessage, error)
}

type PluginTracer struct {
	plugin *plugin.Plugin
	tracer Plugin
}

func NewPluginTracer(path string) (*PluginTracer, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}
	newSym, err := p.Lookup("New")
	if err != nil {
		return nil, err
	}
	newF, ok := newSym.(NewFunc)
	if !ok {
		return nil, errors.New("plugin has invalid new signature")
	}

	t := newF()

	return &PluginTracer{plugin: p, tracer: t}, nil
}

func (t *PluginTracer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) error {
	log.Info("PluginTracer.CaptureStart")
	return nil
}

func (t *PluginTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, memory *vm.Memory, stack *vm.Stack, rStack *vm.ReturnStack, rData []byte, contract *vm.Contract, depth int, err error) error {
	t.tracer.Step(op)
	return nil
}

func (t *PluginTracer) CaptureFault(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, memory *vm.Memory, stack *vm.Stack, rStack *vm.ReturnStack, contract *vm.Contract, depth int, err error) error {
	return nil
}

func (t *PluginTracer) CaptureEnd(output []byte, gasUsed uint64, t_ time.Duration, err error) error {
	return nil
}

func (t *PluginTracer) GetResult() (json.RawMessage, error) {
	return t.tracer.Result()
}
