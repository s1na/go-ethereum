package plugins

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/vm"
)

type CallTracer struct {
}

func NewCallTracer() *CallTracer {
	return &CallTracer{}
}

func (t *CallTracer) Step(_ vm.OpCode) {
	return
}

func (t *CallTracer) Result() (json.RawMessage, error) {
	return nil, nil
}
