package tracers

import (
	"encoding/json"
	"errors"
	"math/big"
	"plugin"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/plugins"
	"github.com/ethereum/go-ethereum/log"
)

type NewFunc = func() PluginAPI

type PluginAPI interface {
	Step(op vm.OpCode)
	Enter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int)
	Exit(output []byte, gasUsed uint64, err error)
	Result(ctx *plugins.PluginContext) (json.RawMessage, error)
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
