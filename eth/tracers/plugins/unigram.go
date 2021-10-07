package main

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

type Tracer struct {
	hist map[vm.OpCode]int
}

func New() tracers.PluginAPI {
	return &Tracer{
		hist: make(map[vm.OpCode]int),
	}
}

func (t *Tracer) Step(op vm.OpCode) {
	if _, ok := t.hist[op]; !ok {
		t.hist[op] = 0
	}
	t.hist[op]++
}

func (t *Tracer) Result() (json.RawMessage, error) {
	return json.Marshal(t.hist)
}
