// Copyright 2025 The go-ethereum Authors
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
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

func init() {
	tracers.LiveDirectory.Register("bal", func(cfg json.RawMessage) (*tracing.Hooks, error) {
		hooks, err := newBALTracer(cfg)
		if err != nil {
			return nil, err
		}
		// Register for state revert events.
		return tracing.WrapWithJournal(hooks)
	})
}

type balanceDiff struct {
	orig *big.Int
	post *big.Int
}

type slotWrite struct {
	orig common.Hash
	post common.Hash
}

type accountAccess struct {
	// Slots reads are aggregated for the whole block.
	SlotReads map[common.Hash]struct{} `json:"slotReads"`
	// Changelog for slots at the tx-level.
	SlotWrites map[common.Hash]map[uint64]*slotWrite `json:"slotWrites"`
	// PreNonces records the nonce prior to a transaction. It is
	// required for contracts that perform CREATE or CREATE2.
	PreNonces map[uint64]*uint64 `json:"nonce"`
	// Changelog for balances at the tx-level.
	BalanceDiffs map[uint64]*balanceDiff `json:"balanceDiffs"`
	// Runtime bytecode of a contract created during block.
	Code         []byte `json:"code"`
	codeHash     common.Hash
	origCodeHash common.Hash
}

type bal = map[common.Address]*accountAccess

// balTracer generates access lists for the blocks.
// See: https://eips.ethereum.org/EIPS/eip-7928
type balTracer struct {
	env   *tracing.VMContext
	al    bal
	txIdx *uint64
}

func newBALTracer(_ json.RawMessage) (*tracing.Hooks, error) {
	t := &balTracer{}
	return &tracing.Hooks{
		OnBlockStart:    t.OnBlockStart,
		OnBlockEnd:      t.OnBlockEnd,
		OnTxStart:       t.OnTxStart,
		OnTxEnd:         t.OnTxEnd,
		OnOpcode:        t.OnOpcode,
		OnNonceChangeV2: t.OnNonceChangeV2,
		OnStorageChange: t.OnStorageChange,
		OnBalanceChange: t.OnBalanceChange,
	}, nil
}

func (t *balTracer) OnOpcode(pc uint64, opcode byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	if err != nil {
		return
	}
	op := vm.OpCode(opcode)
	stackData := scope.StackData()
	stackLen := len(stackData)
	caller := scope.Address()
	switch {
	case stackLen >= 1 && op == vm.SLOAD:
		slot := common.Hash(stackData[stackLen-1].Bytes32())
		t.addSlotRead(caller, slot)
	case stackLen >= 1 && (op == vm.EXTCODECOPY || op == vm.EXTCODEHASH || op == vm.EXTCODESIZE || op == vm.BALANCE || op == vm.SELFDESTRUCT):
		addr := common.Address(stackData[stackLen-1].Bytes20())
		t.addAccount(addr)
	case stackLen >= 5 && (op == vm.DELEGATECALL || op == vm.CALL || op == vm.STATICCALL || op == vm.CALLCODE):
		addr := common.Address(stackData[stackLen-2].Bytes20())
		t.addAccount(addr)
	}
}

func (t *balTracer) OnTxStart(vm *tracing.VMContext, tx *types.Transaction, from common.Address) {
	t.env = vm
	if t.txIdx == nil {
		t.txIdx = new(uint64)
	} else {
		*t.txIdx++
	}
}

func (t *balTracer) OnTxEnd(receipt *types.Receipt, err error) {
	// Iterate the diffs and remove values which have not changed.
	for addr, acc := range t.al {
		if acc.BalanceDiffs != nil {
			txDiff := acc.BalanceDiffs[*t.txIdx]
			if txDiff != nil && txDiff.orig.Cmp(txDiff.post) == 0 {
				delete(acc.BalanceDiffs, *t.txIdx)
			}
		}
		if acc.SlotWrites != nil {
			for _, txes := range acc.SlotWrites {
				txWrite := txes[*t.txIdx]
				if txWrite != nil && txWrite.orig.Cmp(txWrite.post) == 0 {
					delete(txes, *t.txIdx)
					// Still count this as a read.
					t.addSlotRead(addr, txWrite.orig)
				}
			}
		}
		if acc.Code != nil {
			if acc.codeHash == acc.origCodeHash {
				acc.Code = nil
				acc.codeHash = common.Hash{}
				acc.origCodeHash = common.Hash{}
			}
		}
	}
}

func (t *balTracer) OnBlockStart(ev tracing.BlockEvent) {
	t.al = make(bal)
}

func (t *balTracer) OnBlockEnd(err error) {
	if err != nil {
		return
	}
	t.al = nil
	t.txIdx = nil
}

func (t *balTracer) OnNonceChangeV2(addr common.Address, prev, nonce uint64, reason tracing.NonceChangeReason) {
	switch reason {
	case tracing.NonceChangeContractCreator:
		// Don't include nonce for create transactions (i.e. by EoA).
		if t.env.StateDB.GetCodeHash(addr) == (common.Hash{}) {
			return
		}
	case tracing.NonceChangeNewContract:
		// Add nonce in BAL below.
	default:
		return
	}
	t.addAccount(addr)
	if t.al[addr].PreNonces == nil {
		t.al[addr].PreNonces = make(map[uint64]*uint64)
	}
	if t.al[addr].PreNonces[*t.txIdx] == nil {
		t.al[addr].PreNonces[*t.txIdx] = &prev
	}
}

func (t *balTracer) OnStorageChange(addr common.Address, slot common.Hash, prev, post common.Hash) {
	if _, ok := t.al[addr]; !ok {
		t.al[addr] = new(accountAccess)
	}
	if t.al[addr].SlotWrites == nil {
		t.al[addr].SlotWrites = make(map[common.Hash]map[uint64]*slotWrite)
	}
	if t.al[addr].SlotWrites[slot] == nil {
		t.al[addr].SlotWrites[slot] = make(map[uint64]*slotWrite)
		t.al[addr].SlotWrites[slot][*t.txIdx] = &slotWrite{orig: prev}
	}
	t.al[addr].SlotWrites[slot][*t.txIdx].post = post
}

func (t *balTracer) OnBalanceChange(addr common.Address, prev, balance *big.Int, reason tracing.BalanceChangeReason) {
	if _, ok := t.al[addr]; !ok {
		t.al[addr] = new(accountAccess)
	}
	if t.al[addr].BalanceDiffs == nil {
		t.al[addr].BalanceDiffs = make(map[uint64]*balanceDiff)
		t.al[addr].BalanceDiffs[*t.txIdx] = &balanceDiff{orig: prev}
	}
	t.al[addr].BalanceDiffs[*t.txIdx].post = balance
}

func (t *balTracer) OnCodeChange(addr common.Address, prevCodeHash common.Hash, prevCode []byte, codeHash common.Hash, code []byte) {
	if _, ok := t.al[addr]; !ok {
		t.al[addr] = new(accountAccess)
	}
	if t.al[addr].Code == nil {
		t.al[addr].origCodeHash = prevCodeHash
	}
	t.al[addr].codeHash = codeHash
	t.al[addr].Code = code
}

func (t *balTracer) addAccount(addr common.Address) {
	if _, ok := t.al[addr]; !ok {
		t.al[addr] = new(accountAccess)
	}
}

func (t *balTracer) addSlotRead(addr common.Address, slot common.Hash) {
	if _, ok := t.al[addr]; !ok {
		t.al[addr] = new(accountAccess)
	}
	acc := t.al[addr]
	if acc.SlotReads == nil {
		acc.SlotReads = make(map[common.Hash]struct{})
	}
	acc.SlotReads[slot] = struct{}{}
}
