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

func (t *UnigramTracer) Enter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	return
}

func (t *UnigramTracer) Exit(output []byte, gasUsed uint64, err error) {
	return
}

func (t *UnigramTracer) Result(_ *PluginContext) (json.RawMessage, error) {
	return json.Marshal(t.hist)
}

func (t *UnigramTracer) Step(log *StepLog) {
	if _, ok := t.hist[log.Op]; !ok {
		t.hist[log.Op] = 0
	}
	t.hist[log.Op]++
}

func (t *UnigramTracer) Start(ctx *PluginContext, db vm.StateReader) {
}
