package live

import (
	"fmt"
	"math/big"

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

// rewards, withdrawals := supply.Issuance(block, config)
// burn := supply.Burn(block.Header())

// // Calculate the difference between the "calculated" and "crawled" supply delta
// var diff *big.Int
// if crawled != nil {
// 	diff = new(big.Int).Set(crawled)
// 	diff.Sub(diff, rewards)
// 	diff.Sub(diff, withdrawals)
// 	diff.Add(diff, burn)
// }

// burn := new(big.Int)
// if header.BaseFee != nil {
// 	burn = new(big.Int).Mul(new(big.Int).SetUint64(header.GasUsed), header.BaseFee)
// }

type SupplyInfo struct {
	// TODO: rename JSON from supplyDelta to supply
	Delta       *big.Int `json:"supplyDelta"`
	Reward      *big.Int `json:"reward"`
	Withdrawals *big.Int `json:"withdrawals"`
	Burn        *big.Int `json:"burn"`
	// TODO: to be removed
	Destruct *big.Int `json:"destruct"`
}

type SupplyDelta struct {
	SupplyInfo

	Number     uint64      `json:"block"`
	Hash       common.Hash `json:"hash"`
	ParentHash common.Hash `json:"parentHash"`
}

type SupplyTotal struct {
	SupplyInfo

	FromNumber uint64 `json:"fromBlock"`
	ToNumber   uint64 `json:"toBlock"`
}

type Supply struct {
	supply *big.Int
	total  SupplyTotal
	// TODO: rename, delta is not accurate
	delta SupplyDelta

	// Check if genesis has been processed
	hasGenesisProcessed bool
}

func newSupply() (core.BlockchainLogger, error) {
	supplyDelta := newSupplyDelta()

	return &Supply{
		supply: big.NewInt(0),
		total: SupplyTotal{
			SupplyInfo: SupplyInfo{
				Delta:       big.NewInt(0),
				Reward:      big.NewInt(0),
				Withdrawals: big.NewInt(0),
				Burn:        big.NewInt(0),
				Destruct:    big.NewInt(0),
			},
			FromNumber: 0,
			ToNumber:   0,
		},
		delta: supplyDelta,
	}, nil
}

func newSupplyDelta() SupplyDelta {
	return SupplyDelta{
		SupplyInfo: SupplyInfo{
			Delta:       big.NewInt(0),
			Reward:      big.NewInt(0),
			Withdrawals: big.NewInt(0),
			Burn:        big.NewInt(0),
			Destruct:    big.NewInt(0),
		},
		Number:     0,
		Hash:       common.Hash{},
		ParentHash: common.Hash{},
	}
}

func (p *Supply) SetStartBlock(number uint64) {
	if p.total.FromNumber == 0 && p.supply.Cmp(big.NewInt(0)) == 0 {
		p.total.FromNumber = number
	}
}

func (p *Supply) OnBlockStart(b *types.Block, td *big.Int, finalized, safe *types.Header, _ *params.ChainConfig) {
	p.SetStartBlock(b.NumberU64())

	// reset supply delta
	supplyDelta := newSupplyDelta()
	p.delta = supplyDelta

	p.total.ToNumber = b.NumberU64()
}

func (p *Supply) OnBlockEnd(err error) {
	fmt.Printf("OnBlockEnd: err=%v,\t supply=%v\n\t-- delta\t Delta=%v,\t Reward=%v,\t Withdrawals=%v,\t Burn=%v,\t Destruct=%v\n\t-- total\t Delta=%v,\t Reward=%v,\t Withdrawals=%v,\t Burn=%v,\t Destruct=%v\n", err, p.supply, p.delta.Delta, p.delta.Reward, p.delta.Withdrawals, p.delta.Burn, p.delta.Destruct, p.total.Delta, p.total.Reward, p.total.Withdrawals, p.total.Burn, p.total.Destruct)

	fmt.Printf("------------------------------\n\n")
}

func (p *Supply) OnGenesisBlock(b *types.Block, alloc core.GenesisAlloc) {
	if p.hasGenesisProcessed {
		return
	}

	p.SetStartBlock(b.NumberU64())

	// Initialize supply with total allocation in genesis block
	for _, account := range alloc {
		p.supply.Add(p.supply, account.Balance)
	}

	p.hasGenesisProcessed = true

	fmt.Printf("-- OnGenesisBlock: allocLength=%d, supply=%v\n", len(alloc), p.supply)
}

func (p *Supply) OnBalanceChange(a common.Address, prevBalance, newBalance *big.Int, reason state.BalanceChangeReason) {
	diff := new(big.Int).Sub(newBalance, prevBalance)

	fmt.Printf("OnBalanceChange: a=%v, prev=%v, new=%v, \n--\tdiff=%v, reason=%v\n\n", a, prevBalance, newBalance, diff, reason)

	switch reason {
	case state.BalanceIncreaseGenesisBalance:
		p.delta.Delta.Add(p.delta.Delta, diff)
		p.total.Delta.Add(p.total.Delta, diff)
	case state.BalanceIncreaseRewardMineUncle:
	case state.BalanceIncreaseRewardMineBlock:
		p.delta.Reward.Add(p.delta.Reward, diff)
		p.total.Reward.Add(p.total.Reward, diff)
	case state.BalanceChangeWithdrawal:
		p.delta.Withdrawals.Add(p.delta.Withdrawals, diff)
		p.total.Withdrawals.Add(p.total.Withdrawals, diff)
	case state.BalanceDecreaseSelfdestructBurn:
		// TODO: check if diff is not negative and needs Sub, which will affect Delta and Supply as well
		p.delta.Burn.Add(p.delta.Burn, diff)
		p.total.Burn.Add(p.total.Burn, diff)
	default:
		fmt.Printf("No need to take action. Change reason: %v\n", reason)
		return
	}

	// TODO: We might need to check if diff is negative and needs Sub
	p.supply.Add(p.supply, diff)
}

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
