// Copyright 2014 The go-ethereum Authors
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

package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

//go:generate go run github.com/fjl/gencodec -type Log -field-override logMarshaling -out gen_log_json.go

// CaptureStartData stores data when a new transaction starts executing
type CaptureStartData struct {
	From   common.Address  // Sender of the transaction
	To     common.Address  // Recipient of the transaction
	Create bool            // Indicates whether the transaction creates a contract
	Input  []byte          // Input data of the transaction
	Gas    uint64          // Gas provided for the transaction execution
	Value  *big.Int        // Amount of ether transferred in the transaction
}

// CaptureEndData stores data when a transaction finishes executing
type CaptureEndData struct {
	Output  []byte  // Output data from the transaction execution
	GasUsed uint64  // Amount of gas consumed by the transaction
	Err     error   // Error that occurred during transaction execution, if any
}

// CaptureStateData stores data at each step during a transaction's execution
type CaptureStateData struct {
	Pc    uint64             // Current program counter in the EVM code
	Op    vm.OpCode          // Opcode being executed at the current step
	Gas   uint64             // Remaining gas for the transaction
	Cost  uint64             // Cost of the current operation
	Scope *vm.ScopeContext   // Contextual information about the execution environment
	RData []byte             // Return data from executed operations
	Depth int                // Current call depth
	Err   error              // Error that occurred during execution, if any
}

// CaptureFaultData stores data when an execution fault occurs during a transaction's execution
type CaptureFaultData struct {
	Pc    uint64             // Current program counter in the EVM code
	Op    vm.OpCode          // Opcode being executed at the fault
	Gas   uint64             // Remaining gas for the transaction
	Cost  uint64             // Cost of the faulted operation
	Depth int                // Current call depth
	Err   error              // Error that occurred leading to the fault
}

// CaptureKeccakPreimageData stores the data input to the KECCAK256 opcode.
type CaptureKeccakPreimageData struct {
	Hash common.Hash   // The KECCAK256 hash of the data
	Data []byte        // The original data
}

// CaptureEnterData stores data when the EVM enters a new execution scope
type CaptureEnterData struct {
	Type  vm.OpCode        // Opcode that caused the new scope
	From  common.Address   // Address of the scope creator
	To    common.Address   // Address of the new scope
	Input []byte           // Input data to the new scope
	Gas   uint64           // Gas provided to the new scope
	Value *big.Int         // Value transferred into the new scope
}

// CaptureExitData stores data when the EVM exits a scope
type CaptureExitData struct {
	Output  []byte  // Output data from the scope
	GasUsed uint64  // Amount of gas consumed in the scope
	Err     error   // Error that occurred during scope execution, if any
}

// CaptureTxStartData stores data when a transaction begins to execute
type CaptureTxStartData struct {
	Env *vm.EVM              // The EVM environment
	Tx  *Transaction   // The transaction being executed
}

// CaptureTxEndData stores data when a transaction finishes executing
type CaptureTxEndData struct {
	Receipt *Receipt  // The receipt generated from the transaction execution
}

// OnBlockStartData stores data when a new block begins processing
type OnBlockStartData struct {
	Block *Block  // The block being processed
}

// OnBlockEndData stores data when a block has finished processing
type OnBlockEndData struct {
	Td  *big.Int   // Total difficulty of the blockchain after the block
	Err error      // Any error that occurred during block processing
}

// OnBlockValidationErrorData stores data when a block fails validation
type OnBlockValidationErrorData struct {
	Block *Block // The block that failed validation
	Err   error        // The error that caused the validation failure
}

// OnGenesisBlockData stores data when the genesis block is processed
type OnGenesisBlockData struct {
	Block *Block  // The genesis block
}

// OnBalanceChangeData stores data when the balance of an account changes
type OnBalanceChangeData struct {
	Address common.Address  // The account whose balance changed
	Prev    *big.Int        // The previous balance
	New     *big.Int        // The new balance
}

// OnNonceChangeData stores data when the nonce of an account changes
type OnNonceChangeData struct {
	Address common.Address  // The account whose nonce changed
	Prev    uint64          // The previous nonce
	New     uint64          // The new nonce
}

// OnCodeChangeData stores data when the code of an account changes
type OnCodeChangeData struct {
	Address     common.Address  // The account whose code changed
	PrevCodeHash common.Hash    // The KECCAK256 hash of the previous code
	Prev        []byte          // The previous code
	CodeHash    common.Hash     // The KECCAK256 hash of the new code
	Code        []byte          // The new code
}

// OnStorageChangeData stores data when the storage of an account changes
type OnStorageChangeData struct {
	Address common.Address  // The account whose storage changed
	Key     common.Hash     // The storage key that changed
	Prev    common.Hash     // The previous value at the key
	New     common.Hash     // The new value at the key
}

// OnLogData stores data when a new log is created
type OnLogData struct {
	Log *Log  // The new log
}

// OnNewAccountData stores data when a new account is created
type OnNewAccountData struct {
	Address common.Address  // The address of the new account
}

// OnGasConsumedData stores data when gas is consumed during execution
type OnGasConsumedData struct {
	Gas    uint64  // The remaining gas after consumption
	Amount uint64  // The amount of gas consumed
}

// Trace represents chain processing operations. These events are generated by the chain operations.
type Trace struct {
	CaptureStart           []CaptureStartData
	CaptureEnd             []CaptureEndData
	CaptureState           []CaptureStateData
	CaptureFault           []CaptureFaultData
	CaptureKeccakPreimage  []CaptureKeccakPreimageData
	CaptureEnter           []CaptureEnterData
	CaptureExit            []CaptureExitData
	CaptureTxStart         []CaptureTxStartData
	CaptureTxEnd           []CaptureTxEndData
	OnBlockStart           []OnBlockStartData
	OnBlockEnd             []OnBlockEndData
	OnBlockValidationError []OnBlockValidationErrorData
	OnGenesisBlock         []OnGenesisBlockData
	OnBalanceChange        []OnBalanceChangeData
	OnNonceChange          []OnNonceChangeData
	OnCodeChange           []OnCodeChangeData
	OnStorageChange        []OnStorageChangeData
	OnLog                  []OnLogData
	OnNewAccount           []OnNewAccountData
	OnGasConsumed          []OnGasConsumedData
}
