package v2

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
)

// Type aliases so tracers don't have to import both versions.
type (
	// VMContext provides the context for the EVM execution.
	VMContext = tracing.VMContext

	// OpContext provides the context at which the opcode is being
	// executed in, including the memory, stack and various contract-level information.
	OpContext = tracing.OpContext

	// BlockEvent is emitted upon tracing an incoming block.
	// It contains the block as well as consensus related information.
	BlockEvent = tracing.BlockEvent

	// StateDB gives tracers access to the whole state.
	StateDB = tracing.StateDB
)

type (
	// NewLiveTracer is the v2 constructor for a live tracer.
	NewLiveTracer = func(config json.RawMessage) (*Hooks, error)

	// BalanceChangeHook is called when the balance of an account changes.
	BalanceChangeHook = func(addr common.Address, prev, new *big.Int, reason BalanceChangeReason)

	// GasChangeHook is invoked when the gas changes.
	GasChangeHook = func(old, new uint64, reason GasChangeReason)

	// OnSystemCallStartHook is called when a system call is about to be executed. Refer
	// to docs for OnSystemCallStartHook.
	OnSystemCallStartHook = func(vm *tracing.VMContext)

	// BalanceReadHook is called when EVM reads the balance of an account.
	BalanceReadHook = func(addr common.Address, bal *big.Int)

	// NonceReadHook is called when EVM reads the nonce of an account.
	NonceReadHook = func(addr common.Address, nonce uint64)

	// CodeReadHook is called when EVM reads the code of an account.
	CodeReadHook = func(addr common.Address, code []byte)

	// CodeSizeReadHook is called when EVM reads the code size of an account.
	CodeSizeReadHook = func(addr common.Address, size int)

	// CodeHashReadHook is called when EVM reads the code hash of an account.
	CodeHashReadHook = func(addr common.Address, hash common.Hash)

	// StorageReadHook is called when EVM reads a storage slot of an account.
	StorageReadHook = func(addr common.Address, slot, value common.Hash)

	// BlockHashReadHook is called when EVM reads the blockhash of a block.
	BlockHashReadHook = func(blockNumber uint64, hash common.Hash)
)

// Hooks is a collection of hooks in EVM execution, blockchain, and state logic.
// It is used by live tracers which run parallel to the node's execution, as well
// as the debug tracing API.
type Hooks struct {
	// V1 hooks minus OnBlockchainInit which is removed.
	// VM events
	OnTxStart tracing.TxStartHook
	OnTxEnd   tracing.TxEndHook
	OnEnter   tracing.EnterHook
	OnExit    tracing.ExitHook
	OnOpcode  tracing.OpcodeHook
	OnFault   tracing.FaultHook
	// Chain events
	OnBlockchainInit tracing.BlockchainInitHook
	OnClose          tracing.CloseHook
	OnBlockStart     tracing.BlockStartHook
	OnBlockEnd       tracing.BlockEndHook
	OnSkippedBlock   tracing.SkippedBlockHook
	OnGenesisBlock   tracing.GenesisBlockHook
	OnSystemCallEnd  tracing.OnSystemCallEndHook
	// State events
	OnNonceChange   tracing.NonceChangeHook
	OnCodeChange    tracing.CodeChangeHook
	OnStorageChange tracing.StorageChangeHook
	OnLog           tracing.LogHook

	OnBalanceChange BalanceChangeHook
	OnGasChange     GasChangeHook

	// V2 changes
	OnSystemCallStart OnSystemCallStartHook
	// State reads
	OnBalanceRead  BalanceReadHook
	OnNonceRead    NonceReadHook
	OnCodeRead     CodeReadHook
	OnCodeSizeRead CodeSizeReadHook
	OnCodeHashRead CodeHashReadHook
	OnStorageRead  StorageReadHook
	// Block hash read
	OnBlockHashRead BlockHashReadHook
}

// Copy creates a new Hooks instance with all implemented hooks copied from the original.
func (h *Hooks) Copy() *Hooks {
	return tracing.CopyHooks[Hooks, Hooks](h)
}

// ToV2 converts a Hooks instance to a Hooks instance.
//
// Note that OnSystemCallStart hook is excluded from the copy as it is
// changed in a backwards-incompatible way.
func ToV2(h *tracing.Hooks) *Hooks {
	return tracing.CopyHooks[tracing.Hooks, Hooks](h, "OnSystemCallStart", "OnBalanceChange", "OnGasChange")
}

// LiveDirectory is the collection of tracers which can be used
// during normal block import operations.
var LiveDirectory = liveDirectory{elems: make(map[string]NewLiveTracer)}

type liveDirectory struct {
	elems map[string]NewLiveTracer
}

// RegisterV2 registers a tracer constructor by name.
func (d *liveDirectory) Register(name string, f NewLiveTracer) {
	d.elems[name] = f
}

// New instantiates a tracer by name.
func (d *liveDirectory) New(name string, config json.RawMessage) (*Hooks, error) {
	if f, ok := d.elems[name]; ok {
		return f(config)
	}
	return nil, errors.New("not found")
}

// BalanceChangeReason is used to indicate the reason for a balance change, useful
// for tracing and reporting.
type BalanceChangeReason byte

//go:generate go run golang.org/x/tools/cmd/stringer -type=BalanceChangeReason -output gen_balance_change_reason_stringer.go

const (
	BalanceChangeUnspecified BalanceChangeReason = 0

	// Issuance
	// BalanceIncreaseRewardMineUncle is a reward for mining an uncle block.
	BalanceIncreaseRewardMineUncle BalanceChangeReason = 1
	// BalanceIncreaseRewardMineBlock is a reward for mining a block.
	BalanceIncreaseRewardMineBlock BalanceChangeReason = 2
	// BalanceIncreaseWithdrawal is ether withdrawn from the beacon chain.
	BalanceIncreaseWithdrawal BalanceChangeReason = 3
	// BalanceIncreaseGenesisBalance is ether allocated at the genesis block.
	BalanceIncreaseGenesisBalance BalanceChangeReason = 4

	// Transaction fees
	// BalanceIncreaseRewardTransactionFee is the transaction tip increasing block builder's balance.
	BalanceIncreaseRewardTransactionFee BalanceChangeReason = 5
	// BalanceDecreaseGasBuy is spent to purchase gas for execution a transaction.
	// Part of this gas will be burnt as per EIP-1559 rules.
	BalanceDecreaseGasBuy BalanceChangeReason = 6
	// BalanceIncreaseGasReturn is ether returned for unused gas at the end of execution.
	BalanceIncreaseGasReturn BalanceChangeReason = 7

	// DAO fork
	// BalanceIncreaseDaoContract is ether sent to the DAO refund contract.
	BalanceIncreaseDaoContract BalanceChangeReason = 8
	// BalanceDecreaseDaoAccount is ether taken from a DAO account to be moved to the refund contract.
	BalanceDecreaseDaoAccount BalanceChangeReason = 9

	// BalanceChangeTransfer is ether transferred via a call.
	// it is a decrease for the sender and an increase for the recipient.
	BalanceChangeTransfer BalanceChangeReason = 10
	// BalanceChangeTouchAccount is a transfer of zero value. It is only there to
	// touch-create an account.
	BalanceChangeTouchAccount BalanceChangeReason = 11

	// BalanceIncreaseSelfdestruct is added to the recipient as indicated by a selfdestructing account.
	BalanceIncreaseSelfdestruct BalanceChangeReason = 12
	// BalanceDecreaseSelfdestruct is deducted from a contract due to self-destruct.
	BalanceDecreaseSelfdestruct BalanceChangeReason = 13
	// BalanceDecreaseSelfdestructBurn is ether that is sent to an already self-destructed
	// account within the same tx (captured at end of tx).
	// Note it doesn't account for a self-destruct which appoints itself as recipient.
	BalanceDecreaseSelfdestructBurn BalanceChangeReason = 14

	// BalanceChangeRevert is emitted when the balance is reverted back to a previous value due to call failure.
	// It is only emitted when the tracer has opted in to use the journaling wrapper.
	BalanceChangeRevert BalanceChangeReason = 15
)

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
	GasChangeUnspecified GasChangeReason = 0

	// GasChangeTxInitialBalance is the initial balance for the call which will be equal to the gasLimit of the call. There is only
	// one such gas change per transaction.
	GasChangeTxInitialBalance GasChangeReason = 1
	// GasChangeTxIntrinsicGas is the amount of gas that will be charged for the intrinsic cost of the transaction, there is
	// always exactly one of those per transaction.
	GasChangeTxIntrinsicGas GasChangeReason = 2
	// GasChangeTxRefunds is the sum of all refunds which happened during the tx execution (e.g. storage slot being cleared)
	// this generates an increase in gas. There is at most one of such gas change per transaction.
	GasChangeTxRefunds GasChangeReason = 3
	// GasChangeTxLeftOverReturned is the amount of gas left over at the end of transaction's execution that will be returned
	// to the chain. This change will always be a negative change as we "drain" left over gas towards 0. If there was no gas
	// left at the end of execution, no such even will be emitted. The returned gas's value in Wei is returned to caller.
	// There is at most one of such gas change per transaction.
	GasChangeTxLeftOverReturned GasChangeReason = 4

	// GasChangeCallInitialBalance is the initial balance for the call which will be equal to the gasLimit of the call. There is only
	// one such gas change per call.
	GasChangeCallInitialBalance GasChangeReason = 5
	// GasChangeCallLeftOverReturned is the amount of gas left over that will be returned to the caller, this change will always
	// be a negative change as we "drain" left over gas towards 0. If there was no gas left at the end of execution, no such even
	// will be emitted.
	GasChangeCallLeftOverReturned GasChangeReason = 6
	// GasChangeCallLeftOverRefunded is the amount of gas that will be refunded to the call after the child call execution it
	// executed completed. This value is always positive as we are giving gas back to the you, the left over gas of the child.
	// If there was no gas left to be refunded, no such even will be emitted.
	GasChangeCallLeftOverRefunded GasChangeReason = 7
	// GasChangeCallContractCreation is the amount of gas that will be burned for a CREATE.
	GasChangeCallContractCreation GasChangeReason = 8
	// GasChangeContractCreation is the amount of gas that will be burned for a CREATE2.
	GasChangeCallContractCreation2 GasChangeReason = 9
	// GasChangeCallCodeStorage is the amount of gas that will be charged for code storage.
	GasChangeCallCodeStorage GasChangeReason = 10
	// GasChangeCallOpCode is the amount of gas that will be charged for an opcode executed by the EVM, exact opcode that was
	// performed can be check by `OnOpcode` handling.
	GasChangeCallOpCode GasChangeReason = 11
	// GasChangeCallPrecompiledContract is the amount of gas that will be charged for a precompiled contract execution.
	GasChangeCallPrecompiledContract GasChangeReason = 12
	// GasChangeCallStorageColdAccess is the amount of gas that will be charged for a cold storage access as controlled by EIP2929 rules.
	GasChangeCallStorageColdAccess GasChangeReason = 13
	// GasChangeCallFailedExecution is the burning of the remaining gas when the execution failed without a revert.
	GasChangeCallFailedExecution GasChangeReason = 14
	// GasChangeWitnessContractInit flags the event of adding to the witness during the contract creation initialization step.
	GasChangeWitnessContractInit GasChangeReason = 15
	// GasChangeWitnessContractCreation flags the event of adding to the witness during the contract creation finalization step.
	GasChangeWitnessContractCreation GasChangeReason = 16
	// GasChangeWitnessCodeChunk flags the event of adding one or more contract code chunks to the witness.
	GasChangeWitnessCodeChunk GasChangeReason = 17
	// GasChangeWitnessContractCollisionCheck flags the event of adding to the witness when checking for contract address collision.
	GasChangeWitnessContractCollisionCheck GasChangeReason = 18

	// GasChangeIgnored is a special value that can be used to indicate that the gas change should be ignored as
	// it will be "manually" tracked by a direct emit of the gas change event.
	GasChangeIgnored GasChangeReason = 0xFF
)
