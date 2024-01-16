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

type Supply struct {
	delta  SupplyInfo
	logger *log.Logger

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

func (p *Supply) resetDelta() {
	p.delta = newSupplyInfo()
}

func (p *Supply) OnBlockStart(b *types.Block, td *big.Int, finalized, safe *types.Header, _ *params.ChainConfig) {
	p.resetDelta()

	p.delta.Number = b.NumberU64()
	p.delta.Hash = b.Hash()
	p.delta.ParentHash = b.ParentHash()

	// Calculate Burn for this block
	if b.BaseFee() != nil {
		burn := new(big.Int).Mul(new(big.Int).SetUint64(b.GasUsed()), b.BaseFee())
		p.delta.Burn.Add(p.delta.Burn, burn)

		p.delta.Delta.Sub(p.delta.Delta, burn)
	}
}

func (p *Supply) OnBlockEnd(err error) {

	out, _ := json.Marshal(p.delta)
	fmt.Printf("OnBlockEnd: err=%v,\n\t --[supply] %s\n\n", err, out)

	p.logger.Println(string(out))

	fmt.Printf("------------------------------\n\n")
}

func (p *Supply) OnGenesisBlock(b *types.Block, alloc core.GenesisAlloc) {
	if p.hasGenesisProcessed {
		return
	}

	p.resetDelta()

	p.delta.Number = b.NumberU64()
	p.delta.Hash = b.Hash()
	p.delta.ParentHash = b.ParentHash()

	delta := big.NewInt(0)

	// Initialize supply with total allocation in genesis block
	for _, account := range alloc {
		delta.Add(delta, account.Balance)
	}

	p.delta.Delta = delta

	p.hasGenesisProcessed = true

	out, _ := json.Marshal(p.delta)
	fmt.Printf("OnGenesisBlock:\n\t --[supply] %s\n\n", out)

	p.logger.Println(string(out))
}

func (p *Supply) OnBalanceChange(a common.Address, prevBalance, newBalance *big.Int, reason state.BalanceChangeReason) {
	diff := new(big.Int).Sub(newBalance, prevBalance)

	switch reason {
	case state.BalanceIncreaseGenesisBalance:
		p.delta.Delta.Add(p.delta.Delta, diff)
	case state.BalanceIncreaseRewardMineUncle:
	case state.BalanceIncreaseRewardMineBlock:
		p.delta.Reward.Add(p.delta.Reward, diff)
	case state.BalanceChangeWithdrawal:
		p.delta.Withdrawals.Add(p.delta.Withdrawals, diff)
	case state.BalanceDecreaseSelfdestructBurn:
		// TODO: check if diff is not negative and needs Sub, which will affect Delta and Supply as well
		p.delta.Burn.Add(p.delta.Burn, diff)
	default:
		// fmt.Printf("~~\tNo need to take action. Change reason: %v\n\n", reason)
		return
	}

	// TODO: We might need to check if diff is negative and needs Sub
	p.delta.Delta.Add(p.delta.Delta, diff)

	fmt.Printf("\nOnBalanceChange: a=%v, prev=%v, new=%v, \n--\tdiff=%v, reason=%v\n", a, prevBalance, newBalance, diff, reason)
}

// TODO: edge case
// that covers when: contract a selfdestructs, but a receives funds after the selfdestruct opcode executes. Because a is removed only at the end of the transaction the ether sent in between is burnt

// Following methods are not used, but are required to implement the BlockchainLogger interface

// CaptureStart implements the EVMLogger interface to initialize the tracing operation.
func (p *Supply) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (p *Supply) CaptureEnd(output []byte, gasUsed uint64, err error) {}

// CaptureState implements the EVMLogger interface to trace a single step of VM execution.
func (p *Supply) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
}

// CaptureFault implements the EVMLogger interface to trace an execution fault.
func (p *Supply) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
}

// CaptureKeccakPreimage is called during the KECCAK256 opcode.
func (p *Supply) CaptureKeccakPreimage(hash common.Hash, data []byte) {}

// CaptureEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (p *Supply) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {

	// TODO:
	// ------------------
	// Sina, [12/1/24 15:48]
	// - selfdestructing contract names itself as the recipient of funds

	// Sina, [12/1/24 15:48]
	// in this case the ether will be considered burnt

	// Sina, [12/1/24 15:48]
	// This is not captured by any of the balance reasons. Just there is a BalanceIncreaseSelfdestruct but no corresponding BalanceDecreaseSelfdestruct
	// ------------------
}

// CaptureExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
func (p *Supply) CaptureExit(output []byte, gasUsed uint64, err error) {}

func (p *Supply) OnBeaconBlockRootStart(root common.Hash) {}
func (p *Supply) OnBeaconBlockRootEnd()                   {}

func (p *Supply) CaptureTxStart(env *vm.EVM, tx *types.Transaction, from common.Address) {}

func (p *Supply) CaptureTxEnd(receipt *types.Receipt, err error) {}

func (p *Supply) OnNonceChange(a common.Address, prev, new uint64) {}

func (p *Supply) OnCodeChange(a common.Address, prevCodeHash common.Hash, prev []byte, codeHash common.Hash, code []byte) {
}

func (p *Supply) OnStorageChange(a common.Address, k, prev, new common.Hash) {}

func (p *Supply) OnLog(l *types.Log) {}

func (p *Supply) OnNewAccount(a common.Address) {}

func (p *Supply) OnGasChange(old, new uint64, reason vm.GasChangeReason) {}
