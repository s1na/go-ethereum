package live

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"runtime"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/directory"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	directory.LiveDirectory.Register("supply", newSupply)
}

type SupplyInfo struct {
	Delta       *big.Int `json:"delta"`
	Reward      *big.Int `json:"reward"`
	Withdrawals *big.Int `json:"withdrawals"`
	Burn        *big.Int `json:"burn"`

	// Block info
	Number     uint64      `json:"blockNumber"`
	Hash       common.Hash `json:"hash"`
	ParentHash common.Hash `json:"parentHash"`
}

type supplyTxCallstack struct {
	calls []supplyTxCallstack
	burn  *big.Int
}

type Supply struct {
	delta       SupplyInfo
	txCallstack []supplyTxCallstack // Callstack for current transaction
	logger      *log.Logger

	// Check if genesis has been processed
	hasGenesisProcessed bool
}

func newSupply() (core.BlockchainLogger, error) {
	// TODO: file writing has been used for the test and we have to consider if we will write in a file or replace this mechanism at all
	file, err := os.OpenFile("supply.txt", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
	// file, err := os.OpenFile("supply.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	// TODO: better handling of file close
	runtime.SetFinalizer(file, func(f *os.File) {
		f.Close()
	})

	logger := log.New(file, "", 0)

	supplyInfo := newSupplyInfo()

	return &Supply{
		delta:  supplyInfo,
		logger: logger,
	}, nil
}

func newSupplyInfo() SupplyInfo {
	return SupplyInfo{
		Delta:       big.NewInt(0),
		Reward:      big.NewInt(0),
		Withdrawals: big.NewInt(0),
		Burn:        big.NewInt(0),

		Number:     0,
		Hash:       common.Hash{},
		ParentHash: common.Hash{},
	}
}

func (s *Supply) resetDelta() {
	s.delta = newSupplyInfo()
}

func (s *Supply) OnBlockStart(b *types.Block, td *big.Int, finalized, safe *types.Header, _ *params.ChainConfig) {
	s.resetDelta()

	s.delta.Number = b.NumberU64()
	s.delta.Hash = b.Hash()
	s.delta.ParentHash = b.ParentHash()

	// Calculate Burn for this block
	if b.BaseFee() != nil {
		burn := new(big.Int).Mul(new(big.Int).SetUint64(b.GasUsed()), b.BaseFee())
		s.delta.Burn.Add(s.delta.Burn, burn)
		s.delta.Delta.Sub(s.delta.Delta, burn)
	}
}

func (s *Supply) OnBlockEnd(err error) {

	out, _ := json.Marshal(s.delta)
	s.logger.Println(string(out))

	fmt.Printf("OnBlockEnd: err=%v,\n\t --[supply] %s\n\n", err, out)
	fmt.Printf("------------------------------\n\n")
}

func (s *Supply) OnGenesisBlock(b *types.Block, alloc core.GenesisAlloc) {
	if s.hasGenesisProcessed {
		return
	}

	s.resetDelta()

	s.delta.Number = b.NumberU64()
	s.delta.Hash = b.Hash()
	s.delta.ParentHash = b.ParentHash()

	delta := big.NewInt(0)

	// Initialize supply with total allocation in genesis block
	for _, account := range alloc {
		delta.Add(delta, account.Balance)
	}

	s.delta.Delta = delta

	s.hasGenesisProcessed = true

	out, _ := json.Marshal(s.delta)
	s.logger.Println(string(out))

	fmt.Printf("OnGenesisBlock:\n\t --[supply] %s\n\n", out)
}

func (s *Supply) OnBalanceChange(a common.Address, prevBalance, newBalance *big.Int, reason state.BalanceChangeReason) {
	diff := new(big.Int).Sub(newBalance, prevBalance)

	switch reason {
	case state.BalanceIncreaseGenesisBalance:
		s.delta.Delta.Add(s.delta.Delta, diff)
	case state.BalanceIncreaseRewardMineUncle:
	case state.BalanceIncreaseRewardMineBlock:
		s.delta.Reward.Add(s.delta.Reward, diff)
	case state.BalanceIncreaseWithdrawal:
		s.delta.Withdrawals.Add(s.delta.Withdrawals, diff)
	case state.BalanceDecreaseSelfdestructBurn:
		s.delta.Burn.Sub(s.delta.Burn, diff)
	default:
		// fmt.Printf("~~\tNo need to take action. Change reason: %v\n\n", reason)
		return
	}

	s.delta.Delta.Add(s.delta.Delta, diff)

	fmt.Printf("\nOnBalanceChange: a=%v, prev=%v, new=%v, \n--\tdiff=%v, reason=%v\n", a, prevBalance, newBalance, diff, reason)
}

// CaptureStart implements the EVMLogger interface to initialize the tracing operation.
func (s *Supply) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	s.txCallstack = make([]supplyTxCallstack, 1)

	s.txCallstack[0] = supplyTxCallstack{
		calls: make([]supplyTxCallstack, 0),
	}
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (s *Supply) CaptureEnd(output []byte, gasUsed uint64, err error, reverted bool) {
	// No need to handle Burned amount if transaction is reverted
	if !reverted {
		s.interalTxsHandler(&s.txCallstack[0])
	}
}

// interalTxsHandler handles internal transactions burned amount
func (s *Supply) interalTxsHandler(call *supplyTxCallstack) {
	// Handle Burned amount
	if call.burn != nil {
		s.delta.Burn.Add(s.delta.Burn, call.burn)
		s.delta.Delta.Sub(s.delta.Delta, call.burn)
	}

	if len(call.calls) > 0 {
		// Recursivelly handle internal calls
		for _, call := range call.calls {
			callCopy := call
			s.interalTxsHandler(&callCopy)
		}
	}
}

// CaptureEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (s *Supply) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	call := supplyTxCallstack{
		calls: make([]supplyTxCallstack, 0),
	}

	// This is a special case of burned amount which has to be handled here
	// which happens when type == selfdestruct and from == to.
	if typ == vm.SELFDESTRUCT && from == to && value.Cmp(common.Big0) == 1 {
		call.burn = value
	}

	// Append call to the callstack, so we can fill the details in CaptureExit
	s.txCallstack = append(s.txCallstack, call)
}

// CaptureExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
func (s *Supply) CaptureExit(output []byte, gasUsed uint64, err error, reverted bool) {
	size := len(s.txCallstack)
	if size <= 1 {
		return
	}

	// Pop call
	call := s.txCallstack[size-1]
	s.txCallstack = s.txCallstack[:size-1]
	size -= 1

	// In case of a revert, we can drop the call and all its subcalls.
	// Caution, that this has to happen after popping the call from the stack.
	if reverted {
		return
	}
	s.txCallstack[size-1].calls = append(s.txCallstack[size-1].calls, call)
}

// CaptureState implements the EVMLogger interface to trace a single step of VM execution.
func (s *Supply) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
}

// CaptureFault implements the EVMLogger interface to trace an execution fault.
func (s *Supply) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
}

// CaptureKeccakPreimage is called during the KECCAK256 opcode.
func (s *Supply) CaptureKeccakPreimage(hash common.Hash, data []byte) {}

func (s *Supply) OnBeaconBlockRootStart(root common.Hash) {}
func (s *Supply) OnBeaconBlockRootEnd()                   {}

func (s *Supply) CaptureTxStart(env *vm.EVM, tx *types.Transaction, from common.Address) {}

func (s *Supply) CaptureTxEnd(receipt *types.Receipt, err error) {}

func (s *Supply) OnNonceChange(a common.Address, prev, new uint64) {}

func (s *Supply) OnCodeChange(a common.Address, prevCodeHash common.Hash, prev []byte, codeHash common.Hash, code []byte) {
}

func (s *Supply) OnStorageChange(a common.Address, k, prev, new common.Hash) {}

func (s *Supply) OnLog(l *types.Log) {}

func (s *Supply) OnNewAccount(a common.Address) {}

func (s *Supply) OnGasChange(old, new uint64, reason vm.GasChangeReason) {}
