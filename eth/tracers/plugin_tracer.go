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

type NewFunc = func() PluginAPI

type PluginAPI interface {
	Step(op vm.OpCode)
    Enter()
    Exit()
	Result() (json.RawMessage, error)
}

type StepLog struct {
    Pc uint
    Gas uint
    Cost uint
    Depth uint
    Refund uint
    Memory PluginMemoryWrapper
    Contract PluginContractWrapper
    Stack PluginStackWrapper
}

type PluginMemoryWrapper struct {
    memory *vm.Memory
}

func (p *PluginMemoryWrapper) Slice(begin, end int64) []byte {

}

func (p *PluginMemoryWrapper) GetUint(addr uint64) *big.Int {

}

type PluginStackWrapper struct {

}

func (s *PluginStackWrapper) Peek(idx int) *big.Int {

}

type PluginContractWrapper struct {

}

func (c *PluginContractWrapper) Caller() common.Address {

}

func (c *PluginContractWrapper) Address() common.Address {

}

func (c *PluginContractWrapper) Value() *big.Int {

}

func (c *PluginContractWrapper) Input() []byte {

}

type PluginTracer struct {
	plugin *plugin.Plugin
	tracer PluginAPI
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

func (t *PluginTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	log.Info("PluginTracer.CaptureStart")
}

func (t *PluginTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, rData []byte, depth int, err error) {
	t.tracer.Step(op)
}

func (t *PluginTracer) CaptureFault(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
}

func (t *PluginTracer) CaptureEnd(output []byte, gasUsed uint64, t_ time.Duration, err error) {
}

func (t *PluginTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
}

func (t *PluginTracer) CaptureExit(output []byte, gasUsed uint64, err error) {}

func (t *PluginTracer) GetResult() (json.RawMessage, error) {
	return t.tracer.Result()
}
