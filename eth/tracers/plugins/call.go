package plugins

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type CallFrame struct {
	Type    string      `json:"type"`
	From    string      `json:"from"`
	To      string      `json:"to"`
	Input   string      `json:"input"`
	Gas     string      `json:"gas"`
	Value   string      `json:"value,omitempty"`
	GasUsed string      `json:"gasUsed"`
	Output  string      `json:"output"`
	Error   string      `json:"error,omitempty"`
	Calls   []CallFrame `json:"calls,omitempty"`
}

type CallTracer struct {
	callstack []CallFrame
}

func NewCallTracer() *CallTracer {
	t := &CallTracer{callstack: make([]CallFrame, 1)}
	t.callstack[0] = CallFrame{Calls: make([]CallFrame, 0)}
	return t
}

func (t *CallTracer) Step(_ vm.OpCode) {
	return
}

func (t *CallTracer) Enter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	call := CallFrame{
		Type:  typ.String(),
		From:  addrToHex(from),
		To:    addrToHex(to),
		Input: bytesToHex(input),
		Gas:   uintToHex(gas),
		Value: bigToHex(value),
		Calls: make([]CallFrame, 0),
	}
	t.callstack = append(t.callstack, call)
}

func (t *CallTracer) Exit(output []byte, gasUsed uint64, err error) {
	size := len(t.callstack)
	if size > 1 {
		// pop call
		call := t.callstack[size-1]
		t.callstack = t.callstack[:size-1]
		size -= 1

		call.GasUsed = uintToHex(gasUsed)
		if err == nil {
			call.Output = bytesToHex(output)
		} else {
			call.Error = err.Error()
			if call.Type == "CREATE" || call.Type == "CREATE2" {
				call.To = ""
			}
		}
		t.callstack[size-1].Calls = append(t.callstack[size-1].Calls, call)
	}
}

func (t *CallTracer) Result(ctx *PluginContext) (json.RawMessage, error) {
	// Fill in tx context
	callstack := CallFrame{
		Type:    ctx.Type,
		From:    addrToHex(ctx.From),
		To:      addrToHex(ctx.To),
		Input:   bytesToHex(ctx.Input),
		Gas:     uintToHex(ctx.Gas),
		Value:   bigToHex(ctx.Value),
		Output:  bytesToHex(ctx.Output),
		GasUsed: uintToHex(ctx.GasUsed),
	}
	if len(t.callstack[0].Calls) > 0 {
		callstack.Calls = t.callstack[0].Calls
	}
	if ctx.Error != nil {
		callstack.Error = ctx.Error.Error()
	}
	res, err := json.Marshal(callstack)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(res), nil
}

func bytesToHex(s []byte) string {
	return "0x" + common.Bytes2Hex(s)
}

func bigToHex(n *big.Int) string {
	return "0x" + n.Text(16)
}

func uintToHex(n uint64) string {
	return "0x" + strconv.FormatUint(n, 16)
}

func addrToHex(a common.Address) string {
	return strings.ToLower(a.Hex())
}
