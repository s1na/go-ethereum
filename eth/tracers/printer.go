package tracers

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core"

)

type Printer struct {
	bc                         core.BlockChain
	Trace                      types.Trace
	CaptureStartChan           chan types.CaptureStartData
	CaptureEndChan             chan types.CaptureEndData
	CaptureStateChan           chan types.CaptureStateData
	CaptureFaultChan           chan types.CaptureFaultData
	CaptureKeccakPreimageChan  chan types.CaptureKeccakPreimageData
	CaptureEnterChan           chan types.CaptureEnterData
	CaptureExitChan            chan types.CaptureExitData
	CaptureTxStartChan         chan types.CaptureTxStartData
	CaptureTxEndChan           chan types.CaptureTxEndData
	OnBlockStartChan           chan types.OnBlockStartData
	OnBlockEndChan             chan types.OnBlockEndData
	OnBlockValidationErrorChan chan types.OnBlockValidationErrorData
	OnGenesisBlockChan         chan types.OnGenesisBlockData
	OnBalanceChangeChan        chan types.OnBalanceChangeData
	OnNonceChangeChan          chan types.OnNonceChangeData
	OnCodeChangeChan           chan types.OnCodeChangeData
	OnStorageChangeChan        chan types.OnStorageChangeData
	OnLogChan                  chan types.OnLogData
	OnNewAccountChan           chan types.OnNewAccountData
	OnGasConsumedChan          chan types.OnGasConsumedData
	doneChan                   chan struct{}
}

// TODO: Determine the appropriate channel capacity size.
func NewPrinter() *Printer {
	return &Printer{
		Trace:                      types.Trace{},
		CaptureStartChan:           make(chan types.CaptureStartData, 100),
		CaptureEndChan:             make(chan types.CaptureEndData, 100),
		CaptureStateChan:           make(chan types.CaptureStateData, 100),
		CaptureFaultChan:           make(chan types.CaptureFaultData, 100),
		CaptureKeccakPreimageChan:  make(chan types.CaptureKeccakPreimageData, 100),
		CaptureEnterChan:           make(chan types.CaptureEnterData, 100),
		CaptureExitChan:            make(chan types.CaptureExitData, 100),
		CaptureTxStartChan:         make(chan types.CaptureTxStartData, 100),
		CaptureTxEndChan:           make(chan types.CaptureTxEndData, 100),
		OnBlockStartChan:           make(chan types.OnBlockStartData, 100),
		OnBlockEndChan:             make(chan types.OnBlockEndData, 100),
		OnBlockValidationErrorChan: make(chan types.OnBlockValidationErrorData, 100),
		OnGenesisBlockChan:         make(chan types.OnGenesisBlockData, 100),
		OnBalanceChangeChan:        make(chan types.OnBalanceChangeData, 100),
		OnNonceChangeChan:          make(chan types.OnNonceChangeData, 100),
		OnCodeChangeChan:           make(chan types.OnCodeChangeData, 100),
		OnStorageChangeChan:        make(chan types.OnStorageChangeData, 100),
		OnLogChan:                  make(chan types.OnLogData, 100),
		OnNewAccountChan:           make(chan types.OnNewAccountData, 100),
		OnGasConsumedChan:          make(chan types.OnGasConsumedData, 100),
	}
}

// CaptureStart implements the EVMLogger interface to initialize the tracing operation.
func (p *Printer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	// Initialize CaptureStartData with provided arguments
	data := types.CaptureStartData{
		From:  from,
		To:    to,
		Create: create,
		Input: input,
		Gas:   gas,
		Value: value,
	}

	// Send the data to the channel
	p.CaptureStartChan <- data
	fmt.Printf("CaptureStart: from=%v, to=%v, create=%v, input=%v, gas=%v, value=%v\n", from, to, create, input, gas, value)
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (p *Printer) CaptureEnd(output []byte, gasUsed uint64, err error) {
	data := types.CaptureEndData{
		Output:  output,
		GasUsed: gasUsed,
		Err:     err,
	}
	p.CaptureEndChan <- data
	fmt.Printf("CaptureEnd: output=%v, gasUsed=%v, err=%v\n", output, gasUsed, err)
}

// CaptureState implements the EVMLogger interface to trace a single step of VM execution.
func (p *Printer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
/* 	data := types.CaptureStateData{
		Pc:    pc,
		Op:    op,
		Gas:   gas,
		Cost:  cost,
		Scope: scope,
		RData: rData,
		Depth: depth,
		Err:   err,
	}
	p.CaptureStateChan <- data */
	//fmt.Printf("CaptureState: pc=%v, op=%v, gas=%v, cost=%v, scope=%v, rData=%v, depth=%v, err=%v\n", pc, op, gas, cost, scope, rData, depth, err)
}

// CaptureFault implements the EVMLogger interface to trace an execution fault.
func (p *Printer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
	data := types.CaptureFaultData{
		Pc:    pc,
		Op:    op,
		Gas:   gas,
		Cost:  cost,
		Depth: depth,
		Err:   err,
	}
	p.CaptureFaultChan <- data
	fmt.Printf("CaptureFault: pc=%v, op=%v, gas=%v, cost=%v, depth=%v, err=%v\n", pc, op, gas, cost, depth, err)
}

// CaptureKeccakPreimage is called during the KECCAK256 opcode.
func (p *Printer) CaptureKeccakPreimage(hash common.Hash, data []byte) {
/* 	preImageData := types.CaptureKeccakPreimageData{
		Hash: hash,
		Data: data,
	}
	p.CaptureKeccakPreimageChan <- preImageData */
}

// CaptureEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (p *Printer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	data := types.CaptureEnterData{
		Type:   typ,
		From:  from,
		To:    to,
		Input: input,
		Gas:   gas,
		Value: value,
	}
	p.CaptureEnterChan <- data
	fmt.Printf("CaptureEnter: typ=%v, from=%v, to=%v, input=%v, gas=%v, value=%v\n", typ, from, to, input, gas, value)
}

// CaptureExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
func (p *Printer) CaptureExit(output []byte, gasUsed uint64, err error) {
	data := types.CaptureExitData{
		Output:  output,
		GasUsed: gasUsed,
		Err:     err,
	}
	p.CaptureExitChan <- data
	fmt.Printf("CaptureExit: output=%v, gasUsed=%v, err=%v\n", output, gasUsed, err)
}

func (p *Printer) CaptureTxStart(env *vm.EVM, tx *types.Transaction) {
	data := types.CaptureTxStartData{
		Env: env,
		Tx:  tx,
	}
	p.CaptureTxStartChan <- data
	fmt.Printf("CaptureTxStart: tx=%v\n", tx)

}

func (p *Printer) CaptureTxEnd(receipt *types.Receipt) {
	data := types.CaptureTxEndData{
		Receipt: receipt,
	}
	p.CaptureTxEndChan <- data
	fmt.Printf("CaptureTxEnd: receipt=%v\n", receipt)
}

func (p *Printer) OnBlockStart(b *types.Block) {
	data := types.OnBlockStartData{
		Block: b,
	}
	p.OnBlockStartChan <- data
	fmt.Printf("OnBlockStart: b=%v\n", b.NumberU64())
}

func (p *Printer) OnBlockEnd(td *big.Int, err error) {
	data := types.OnBlockEndData{
		Td:  td,
		Err: err,
	}
	p.OnBlockEndChan <- data
	fmt.Printf("OnBlockEnd: td=%v, err=%v\n", td, err)
}

func (p *Printer) OnBlockValidationError(block *types.Block, err error) {
	data := types.OnBlockValidationErrorData{
		Block: block,
		Err:   err,
	}
	p.OnBlockValidationErrorChan <- data
	fmt.Printf("OnBlockValidationError: b=%v, err=%v\n", block.NumberU64(), err)
}

func (p *Printer) OnGenesisBlock(b *types.Block) {
	data := types.OnGenesisBlockData{
		Block: b,
	}
	p.OnGenesisBlockChan <- data
	fmt.Printf("OnGenesisBlock: b=%v\n", b.NumberU64())
}

func (p *Printer) OnBalanceChange(a common.Address, prev, new *big.Int) {
	data := types.OnBalanceChangeData{
		Address: a,
		Prev: prev,
		New:  new,
	}
	p.OnBalanceChangeChan <- data
	fmt.Printf("OnBalanceChange: a=%v, prev=%v, new=%v\n", a, prev, new)
}

func (p *Printer) OnNonceChange(a common.Address, prev, new uint64) {
	data := types.OnNonceChangeData{
		Address: a,
		Prev: prev,
		New:  new,
	}
	p.OnNonceChangeChan <- data
	fmt.Printf("OnNonceChange: a=%v, prev=%v, new=%v\n", a, prev, new)
}

func (p *Printer) OnCodeChange(a common.Address, prevCodeHash common.Hash, prev []byte, codeHash common.Hash, code []byte) {
	data := types.OnCodeChangeData{
		Address: a,
		PrevCodeHash: prevCodeHash,
		Prev:         prev,
		CodeHash:     codeHash,
		Code:         code,
	}
	p.OnCodeChangeChan <- data
	fmt.Printf("OnCodeChange: a=%v, prevCodeHash=%v, prev=%v, codeHash=%v, code=%v\n", a, prevCodeHash, prev, codeHash, code)
}

func (p *Printer) OnStorageChange(a common.Address, k, prev, new common.Hash) {
	data := types.OnStorageChangeData{
		Address: a,
		Key: k,
		Prev: prev,
		New:  new,
	}
	p.OnStorageChangeChan <- data
	fmt.Printf("OnStorageChange: a=%v, k=%v, prev=%v, new=%v\n", a, k, prev, new)
}

func (p *Printer) OnLog(l *types.Log) {
	data := types.OnLogData{
		Log: l,
	}
	p.OnLogChan <- data
	fmt.Printf("OnLog: l=%v\n", l)
}

func (p *Printer) OnNewAccount(a common.Address) {
	data := types.OnNewAccountData{
		Address: a,
	}
	p.OnNewAccountChan <- data
	fmt.Printf("OnNewAccount: a=%v\n", a)
}

func (p *Printer) OnGasConsumed(gas, amount uint64) {
	data := types.OnGasConsumedData{
		Gas:    gas,
		Amount: amount,
	}
	p.OnGasConsumedChan <- data
	fmt.Printf("OnGasConsumed: gas=%v, amount=%v\n", gas, amount)
}

// EventLoop receives data from channels, adds them to Trace,
// and sends Trace when the OnBlockEnd event occurs. This function operates
// in a loop and should typically be run in a separate goroutine.
func (p *Printer) EventLoop(bc core.BlockChain) {
	for {
		select {
		case data := <-p.CaptureStartChan:
			p.Trace.CaptureStart = append(p.Trace.CaptureStart, data)
		case data := <-p.CaptureEndChan:
			p.Trace.CaptureEnd = append(p.Trace.CaptureEnd, data)
		case data := <-p.CaptureStateChan:
			p.Trace.CaptureState = append(p.Trace.CaptureState, data)
		case data := <-p.CaptureFaultChan:
			p.Trace.CaptureFault = append(p.Trace.CaptureFault, data)
		case data := <-p.CaptureKeccakPreimageChan:
			p.Trace.CaptureKeccakPreimage = append(p.Trace.CaptureKeccakPreimage, data)
		case data := <-p.CaptureEnterChan:
			p.Trace.CaptureEnter = append(p.Trace.CaptureEnter, data)
		case data := <-p.CaptureExitChan:
			p.Trace.CaptureExit = append(p.Trace.CaptureExit, data)
		case data := <-p.CaptureTxStartChan:
			p.Trace.CaptureTxStart = append(p.Trace.CaptureTxStart, data)
		case data := <-p.CaptureTxEndChan:
			p.Trace.CaptureTxEnd = append(p.Trace.CaptureTxEnd, data)
		case data := <-p.OnBlockStartChan:
			p.Trace.OnBlockStart = append(p.Trace.OnBlockStart, data)
		case data := <-p.OnBlockEndChan:
			p.Trace.OnBlockEnd = append(p.Trace.OnBlockEnd, data)
        	bc.tracesFeed.send(p.Trace)
			p.Trace = types.Trace{}
		case data := <-p.OnBlockValidationErrorChan:
			p.Trace.OnBlockValidationError = append(p.Trace.OnBlockValidationError, data)
		case data := <-p.OnGenesisBlockChan:
			p.Trace.OnGenesisBlock = append(p.Trace.OnGenesisBlock, data)
		case data := <-p.OnBalanceChangeChan:
			p.Trace.OnBalanceChange = append(p.Trace.OnBalanceChange, data)
		case data := <-p.OnNonceChangeChan:
			p.Trace.OnNonceChange = append(p.Trace.OnNonceChange, data)
		case data := <-p.OnCodeChangeChan:
			p.Trace.OnCodeChange = append(p.Trace.OnCodeChange, data)
		case data := <-p.OnStorageChangeChan:
			p.Trace.OnStorageChange = append(p.Trace.OnStorageChange, data)
		case data := <-p.OnLogChan:
			p.Trace.OnLog = append(p.Trace.OnLog, data)
		case data := <-p.OnNewAccountChan:
			p.Trace.OnNewAccount = append(p.Trace.OnNewAccount, data)
		case data := <-p.OnGasConsumedChan:
			p.Trace.OnGasConsumed = append(p.Trace.OnGasConsumed, data)
		case <-bc.quit:
			return

		}
	}
}
