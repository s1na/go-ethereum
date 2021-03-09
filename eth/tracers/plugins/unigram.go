package main

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/vm"
)

type Tracer struct {
	hist map[vm.OpCode]int
}

func New() *Tracer {
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
