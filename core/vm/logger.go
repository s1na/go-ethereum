// Copyright 2015 The go-ethereum Authors
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

package vm

import (
	"errors"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// EVMLogger is used to collect execution traces from an EVM transaction
// execution. CaptureState is called for each step of the VM with the
// current VM state.
// Note that reference types are actual VM data structures; make copies
// if you need to retain them beyond the current call.
type EVMLogger interface {
	// Transaction level
	CaptureTxStart(evm *EVM, tx *types.Transaction)
	CaptureTxEnd(receipt *types.Receipt, err error)
	// Top call frame
	CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int)
	// PR: I left the signature as-is, code can be retrieved using `if v := (*vm.CallError); errors.As(err, &v) { v.Code() })`.
	//     This avoids signature change but brings two cons, `err == vm.ErrOutOfGas` doesn't work anymore (errors.Is(err, vm.ErrOutOfGas) still works),
	//     second the "CallError" is hidden and you need to explicitly cast to it to get the code.
	//
	// Other possibilities we could do
	//  - `CaptureEnd(output []byte, gasUsed uint64, err *CallError)`
	//  - `CaptureEnd(output []byte, gasUsed uint64, err error, errCode CallErrorCode)`
	//
	// Other wilder possibilities would be to add `Code()` straight to `vm.ErrXYZ` errors with the introduction
	// of a `VMErr` interface, internally this would mean also `err == vm.ErrOutOfGas` wouldn't work anymore (would need
	// to ne converted to `errors.Is(...)`).
	//
	// The comment above about the signature change also applies to `CaptureExit`, `CaptureState` and `CaptureFault`.
	CaptureEnd(output []byte, gasUsed uint64, err error)
	// Rest of call frames
	CaptureEnter(typ OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int)
	CaptureExit(output []byte, gasUsed uint64, err error)
	// Opcode level
	CaptureState(pc uint64, op OpCode, gas, cost uint64, scope *ScopeContext, rData []byte, depth int, err error)
	CaptureFault(pc uint64, op OpCode, gas, cost uint64, scope *ScopeContext, depth int, err error)
	CaptureKeccakPreimage(hash common.Hash, data []byte)
	// Misc
	OnGasChange(old, new uint64, reason GasChangeReason)
}

// PR: Maybe this should be tied from explicitely to "tracer" like `EVMLoggerCallError`
// alternatives
//
// PR2: Need to decide where we put all those new stuff
type CallError struct {
	error
	code CallErrorCode
}

func CallErrorFromErr(err error) error {
	if err == nil {
		return nil
	}

	return &CallError{
		error: err,
		code:  callErrorCodeFromErr(err),
	}
}

func (e *CallError) Error() string {
	return e.error.Error()
}

func (e *CallError) Unwrap() error {
	return errors.Unwrap(e.error)
}

func (e *CallError) Code() CallErrorCode {
	return e.code
}

type CallErrorCode int

const (
	CallErrorCodeUnspecified CallErrorCode = iota

	CallErrorCodeOutOfGas
	CallErrorCodeCodeStoreOutOfGas
	CallErrorCodeDepth
	CallErrorCodeInsufficientBalance
	CallErrorCodeContractAddressCollision
	CallErrorCodeExecutionReverted
	CallErrorCodeMaxInitCodeSizeExceeded
	CallErrorCodeMaxCodeSizeExceeded
	CallErrorCodeInvalidJump
	CallErrorCodeWriteProtection
	CallErrorCodeReturnDataOutOfBounds
	CallErrorCodeGasUintOverflow
	CallErrorCodeInvalidCode
	CallErrorCodeNonceUintOverflow
	CallErrorCodeStackUnderflow
	CallErrorCodeStackOverflow
	CallErrorCodeInvalidOpCode

	// CallErrorCodeUnknown explicitly marks an error as unknown, this is useful when error is converted
	// from an actual `error` in which case if the mapping is not known, we can use this value to indicate that.
	CallErrorCodeUnknown = math.MaxInt - 1
)

func callErrorCodeFromErr(err error) CallErrorCode {
	switch {
	case errors.Is(err, ErrOutOfGas):
		return CallErrorCodeOutOfGas
	case errors.Is(err, ErrCodeStoreOutOfGas):
		return CallErrorCodeCodeStoreOutOfGas
	case errors.Is(err, ErrDepth):
		return CallErrorCodeDepth
	case errors.Is(err, ErrInsufficientBalance):
		return CallErrorCodeInsufficientBalance
	case errors.Is(err, ErrContractAddressCollision):
		return CallErrorCodeContractAddressCollision
	case errors.Is(err, ErrExecutionReverted):
		return CallErrorCodeExecutionReverted
	case errors.Is(err, ErrMaxInitCodeSizeExceeded):
		return CallErrorCodeMaxInitCodeSizeExceeded
	case errors.Is(err, ErrMaxCodeSizeExceeded):
		return CallErrorCodeMaxCodeSizeExceeded
	case errors.Is(err, ErrInvalidJump):
		return CallErrorCodeInvalidJump
	case errors.Is(err, ErrWriteProtection):
		return CallErrorCodeWriteProtection
	case errors.Is(err, ErrReturnDataOutOfBounds):
		return CallErrorCodeReturnDataOutOfBounds
	case errors.Is(err, ErrGasUintOverflow):
		return CallErrorCodeGasUintOverflow
	case errors.Is(err, ErrInvalidCode):
		return CallErrorCodeInvalidCode
	case errors.Is(err, ErrNonceUintOverflow):
		return CallErrorCodeNonceUintOverflow

	default:
		// Dynamic errors
		if v := (*ErrStackUnderflow)(nil); errors.As(err, &v) {
			return CallErrorCodeStackUnderflow
		}

		if v := (*ErrStackOverflow)(nil); errors.As(err, &v) {
			return CallErrorCodeStackOverflow
		}

		if v := (*ErrInvalidOpCode)(nil); errors.As(err, &v) {
			return CallErrorCodeInvalidOpCode
		}

		return CallErrorCodeUnknown
	}
}

// GasChangeReason is used to indicate the reason for a gas change, useful
// for tracing and reporting.
//
// There is essentially two types of gas changes, those that can be emitted once per transaction
// and those that can be emitted on a call basis, so possibly multiple times per transaction.
//
// They can be recognized easily by their name, those that start with `GasChangeTx` are emitted
// once per transaction, while those that start with `GasChangeCall` are emitted on a call basis.
type GasChangeReason byte

const (
	GasChangeUnspecified GasChangeReason = iota

	// GasChangeTxInitialBalance is the initial balance for the call which will be equal to the gasLimit of the call. There is only
	// one such gas change per transaction.
	GasChangeTxInitialBalance
	// GasChangeTxIntrinsicGas is the amount of gas that will be charged for the intrinsic cost of the transaction, there is
	// always exactly one of those per transaction.
	GasChangeTxIntrinsicGas
	// GasChangeTxRefunds is the sum of all refunds which happened during the tx execution (e.g. storage slot being cleared)
	// this generates an increase in gas. There is at most one of such gas change per transaction.
	GasChangeTxRefunds
	// GasChangeTxLeftOverReturned is the amount of gas left over at the end of transaction's execution that will be returned
	// to the chain. This change will always be a negative change as we "drain" left over gas towards 0. If there was no gas
	// left at the end of execution, no such even will be emitted. The returned gas's value in Wei is returned to caller.
	// There is at most one of such gas change per transaction.
	GasChangeTxLeftOverReturned

	// GasChangeCallInitialBalance is the initial balance for the call which will be equal to the gasLimit of the call. There is only
	// one such gas change per call.
	GasChangeCallInitialBalance
	// GasChangeCallLeftOverReturned is the amount of gas left over that will be returned to the caller, this change will always
	// be a negative change as we "drain" left over gas towards 0. If there was no gas left at the end of execution, no such even
	// will be emitted.
	GasChangeCallLeftOverReturned
	// GasChangeCallLeftOverRefunded is the amount of gas that will be refunded to the call after the child call execution it
	// executed completed. This value is always positive as we are giving gas back to the you, the left over gas of the child.
	// If there was no gas left to be refunded, no such even will be emitted.
	GasChangeCallLeftOverRefunded
	// GasChangeCallContractCreation is the amount of gas that will be burned for a CREATE.
	GasChangeCallContractCreation
	// GasChangeContractCreation is the amount of gas that will be burned for a CREATE2.
	GasChangeCallContractCreation2
	// GasChangeCallCodeStorage is the amount of gas that will be charged for code storage.
	GasChangeCallCodeStorage
	// GasChangeCallOpCode is the amount of gas that will be charged for an opcode executed by the EVM, exact opcode that was
	// performed can be check by `CaptureState` handling.
	GasChangeCallOpCode
	// GasChangeCallPrecompiledContract is the amount of gas that will be charged for a precompiled contract execution.
	GasChangeCallPrecompiledContract
	// GasChangeCallStorageColdAccess is the amount of gas that will be charged for a cold storage access as controlled by EIP2929 rules.
	GasChangeCallStorageColdAccess
	// GasChangeCallFailedExecution is the burning of the remaining gas when the execution failed without a revert.
	GasChangeCallFailedExecution

	// GasChangeIgnored is a special value that can be used to indicate that the gas change should be ignored as
	// it will be "manually" tracked by a direct emit of the gas change event.
	GasChangeIgnored GasChangeReason = 0xFF
)
