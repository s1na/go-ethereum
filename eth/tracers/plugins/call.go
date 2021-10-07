package plugins

import (
	"encoding/json"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type CallFrame struct {
	Type    string
	From    string
	To      string
	Input   string
	Gas     string
	Value   string `json:",omitempty"`
	GasUsed string
	Output  string
	Error   string      `json:",omitempty"`
	Calls   []CallFrame `json:",omitempty"`
}

type CallTracer struct {
	callstack []CallFrame
}

func NewCallTracer() *CallTracer {
	return &CallTracer{callstack: make([]CallFrame, 0)}
}

func (t *CallTracer) Step(_ vm.OpCode) {
	return
}

func (t *CallTracer) Enter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	call := CallFrame{
		Type:  typ.String(),
		From:  from.Hex(),
		To:    to.Hex(),
		Input: bytesToHex(input),
		Gas:   strconv.FormatUint(gas, 16),
		Value: value.Text(16),
		Calls: make([]CallFrame, 0),
	}
	t.callstack = append(t.callstack, call)
}

func (t *CallTracer) Exit(output []byte, gasUsed uint64, err error) {
	size := len(t.callstack)
	if size > 1 {
		call := t.callstack[size-1]
		t.callstack = t.callstack[:size-1]
		call.GasUsed = strconv.FormatUint(gasUsed, 16)
		if err == nil {
			call.Output = bytesToHex(output)
		} else {
			call.Error = err.Error()
			if call.Type == "CREATE" || call.Type == "CREATE2" {
				call.To = ""
			}
		}
		size -= 1
		t.callstack[size-1].Calls = append(t.callstack[size-1].Calls, call)
	}
}

func (t *CallTracer) Result() (json.RawMessage, error) {
	// Fill in tx context
	callstack := CallFrame{
		Calls: t.callstack,
	}
	res, err := json.Marshal(callstack)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(res), nil
}

func bytesToHex(s []byte) string {
	a := "0x" + common.Bytes2Hex(s)
	return a
}
