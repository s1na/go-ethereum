package plugins

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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

func (t *UnigramTracer) Enter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	return
}

func (t *UnigramTracer) Exit(output []byte, gasUsed uint64, err error) {
	return
}

func (t *UnigramTracer) Result() (json.RawMessage, error) {
	return json.Marshal(t.hist)
}
