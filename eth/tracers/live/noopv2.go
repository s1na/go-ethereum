// Copyright 2024 The go-ethereum Authors
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

package live

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	tracers.LiveDirectory.RegisterV2("noopv2", newNoopV2Tracer)
}

// noopV2 is a no-op live tracer. It's there to
// catch changes in the tracing interface, as well as
// for testing live tracing performance. Can be removed
// as soon as we have a real live tracer.
type noopV2 struct{}

func newNoopV2Tracer(_ json.RawMessage) (*tracing.HooksV2, error) {
	t := &noopV2{}
	return &tracing.HooksV2{
		OnTxStart:         t.OnTxStart,
		OnTxEnd:           t.OnTxEnd,
		OnEnter:           t.OnEnter,
		OnExit:            t.OnExit,
		OnOpcode:          t.OnOpcode,
		OnFault:           t.OnFault,
		OnGasChange:       t.OnGasChange,
		OnBlockchainInit:  t.OnBlockchainInit,
		OnBlockStart:      t.OnBlockStart,
		OnBlockEnd:        t.OnBlockEnd,
		OnSkippedBlock:    t.OnSkippedBlock,
		OnGenesisBlock:    t.OnGenesisBlock,
		OnSystemCallStart: t.OnSystemCallStart,
		OnSystemCallEnd:   t.OnSystemCallEnd,
		OnReorg:           t.OnReorg,
		OnBalanceChange:   t.OnBalanceChange,
		OnNonceChange:     t.OnNonceChange,
		OnCodeChange:      t.OnCodeChange,
		OnStorageChange:   t.OnStorageChange,
		OnLog:             t.OnLog,
		OnBalanceRead:     t.OnBalanceRead,
		OnNonceRead:       t.OnNonceRead,
		OnCodeRead:        t.OnCodeRead,
		OnCodeSizeRead:    t.OnCodeSizeRead,
		OnCodeHashRead:    t.OnCodeHashRead,
		OnStorageRead:     t.OnStorageRead,
		OnBlockHashRead:   t.OnBlockHashRead,
	}, nil
}

func (t *noopV2) OnOpcode(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
}

func (t *noopV2) OnFault(pc uint64, op byte, gas, cost uint64, _ tracing.OpContext, depth int, err error) {
}

func (t *noopV2) OnEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
}

func (t *noopV2) OnExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
}

func (t *noopV2) OnTxStart(vm *tracing.VMContext, tx *types.Transaction, from common.Address) {
}

func (t *noopV2) OnTxEnd(receipt *types.Receipt, err error) {
}

func (t *noopV2) OnBlockStart(ev tracing.BlockEvent) {
}

func (t *noopV2) OnBlockEnd(err error) {
}

func (t *noopV2) OnSkippedBlock(ev tracing.BlockEvent) {}

func (t *noopV2) OnBlockchainInit(chainConfig *params.ChainConfig) {
}

func (t *noopV2) OnGenesisBlock(b *types.Block, alloc types.GenesisAlloc) {
}

func (t *noopV2) OnSystemCallStart(ctx *tracing.VMContext) {}

func (t *noopV2) OnSystemCallEnd() {}

func (t *noopV2) OnReorg(reverted []*types.Block) {}

func (t *noopV2) OnBalanceChange(a common.Address, prev, new *big.Int, reason tracing.BalanceChangeReason) {
}

func (t *noopV2) OnNonceChange(a common.Address, prev, new uint64) {
}

func (t *noopV2) OnCodeChange(a common.Address, prevCodeHash common.Hash, prev []byte, codeHash common.Hash, code []byte) {
}

func (t *noopV2) OnStorageChange(a common.Address, k, prev, new common.Hash) {
}

func (t *noopV2) OnLog(l *types.Log) {

}

func (t *noopV2) OnBalanceRead(addr common.Address, bal *big.Int) {}

func (t *noopV2) OnNonceRead(addr common.Address, nonce uint64) {}

func (t *noopV2) OnCodeRead(addr common.Address, code []byte) {}

func (t *noopV2) OnCodeSizeRead(addr common.Address, size int) {}

func (t *noopV2) OnCodeHashRead(addr common.Address, hash common.Hash) {}

func (t *noopV2) OnStorageRead(addr common.Address, slot, val common.Hash) {}

func (t *noopV2) OnBlockHashRead(number uint64, hash common.Hash) {}

func (t *noopV2) OnGasChange(old, new uint64, reason tracing.GasChangeReason) {}
