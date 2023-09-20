// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package logger

import (
	"encoding/json"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
)

type JSONLogger struct {
	encoder *json.Encoder
	cfg     *Config
	env     *vm.EVM
	prev    *stepMarshalling
}

type stepMarshalling struct {
	PC            uint64              `json:"pc"`
	Op            vm.OpCode           `json:"op"`
	Gas           math.HexOrDecimal64 `json:"gas"`
	GasCost       math.HexOrDecimal64 `json:"gasCost"`
	Memory        json.RawMessage     `json:"memory,omitempty"`
	MemorySize    int                 `json:"memorySize"`
	Stack         json.RawMessage     `json:"stack,omitempty"`
	ReturnData    json.RawMessage     `json:"returnData,omitempty"`
	Depth         int                 `json:"depth"`
	RefundCounter uint64              `json:"refundCounter"`
	Err           string              `json:"error,omitempty"`
}

// NewJSONLogger creates a new EVM tracer that prints execution steps as JSON objects
// into the provided stream.
func NewJSONLogger(cfg *Config, writer io.Writer) *JSONLogger {
	l := &JSONLogger{encoder: json.NewEncoder(writer), cfg: cfg}
	if l.cfg == nil {
		l.cfg = &Config{}
	}
	return l
}

func (l *JSONLogger) CaptureStart(env *vm.EVM, from, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	l.env = env
}

func (l *JSONLogger) CaptureFault(pc uint64, op vm.OpCode, gas uint64, cost uint64, scope *vm.ScopeContext, depth int, err error) {
	if l.prev == nil {
		return
	}
	if err != nil {
		l.prev.Err = err.Error()
	}
	l.encoder.Encode(l.prev)
	l.prev = nil
}

// CaptureState outputs state information on the logger.
func (l *JSONLogger) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	if l.prev != nil {
		l.encoder.Encode(l.prev)
	}
	// Partially encode the log entry.
	// This is to avoid copying memory and stack.
	step := stepMarshalling{
		PC:            pc,
		Op:            op,
		Gas:           math.HexOrDecimal64(gas),
		GasCost:       math.HexOrDecimal64(cost),
		MemorySize:    scope.Memory.Len(),
		Depth:         depth,
		RefundCounter: l.env.StateDB.GetRefund(),
	}
	if err != nil {
		step.Err = err.Error()
	}
	memory := scope.Memory
	stack := scope.Stack
	if l.cfg.EnableMemory {
		mem, err := json.Marshal(hexutil.Bytes(memory.Data()))
		if err != nil {
			return
		}
		step.Memory = mem
	}
	if !l.cfg.DisableStack {
		stack, err := json.Marshal(stack.Data())
		if err != nil {
			return
		}
		step.Stack = stack
	}
	if l.cfg.EnableReturnData {
		data, err := json.Marshal(hexutil.Bytes(rData))
		if err != nil {
			return
		}
		step.ReturnData = data
	}
	l.prev = &step
}

// CaptureEnd is triggered at end of execution.
func (l *JSONLogger) CaptureEnd(output []byte, gasUsed uint64, err error) {
	if l.prev != nil {
		l.encoder.Encode(l.prev)
	}
	type endLog struct {
		Output  string              `json:"output"`
		GasUsed math.HexOrDecimal64 `json:"gasUsed"`
		Err     string              `json:"error,omitempty"`
	}
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	l.encoder.Encode(endLog{common.Bytes2Hex(output), math.HexOrDecimal64(gasUsed), errMsg})
}

func (l *JSONLogger) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
}

func (l *JSONLogger) CaptureExit(output []byte, gasUsed uint64, err error) {}

func (l *JSONLogger) CaptureTxStart(gasLimit uint64) {}

func (l *JSONLogger) CaptureTxEnd(restGas uint64) {}
