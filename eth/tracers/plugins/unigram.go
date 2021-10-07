package plugins

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/vm"
)

type UnigramTracer struct {
	hist map[vm.OpCode]int
}

func NewUnigramTracer() *UnigramTracer {
	return &UnigramTracer{
		hist: make(map[vm.OpCode]int),
	}
}

func (t *UnigramTracer) Step(op vm.OpCode) {
	if _, ok := t.hist[op]; !ok {
		t.hist[op] = 0
	}
	t.hist[op]++
}

func (t *UnigramTracer) Result() (json.RawMessage, error) {
	return json.Marshal(t.hist)
}
