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

package state

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/holiman/uint256"
)

// hookedStateDB represents a statedb which emits calls to tracing-hooks
// on state operations.
type hookedStateDB struct {
	inner *StateDB
	hooks *tracing.Hooks
}

// NewHookedState wraps the given stateDb with the given hooks
func NewHookedState(stateDb *StateDB, hooks *tracing.Hooks) *hookedStateDB {
	s := &hookedStateDB{stateDb, hooks}
	if s.hooks == nil {
		s.hooks = new(tracing.Hooks)
	}
	s.inner.reader = newHookedReader(s.inner.reader, s.hooks)
	return s
}

// Close unwraps the inner reader.
func (s *hookedStateDB) Close() {
	if rd, ok := s.inner.reader.(*hookedReader); ok {
		s.inner.reader = rd.inner
	}
}

func (s *hookedStateDB) CreateAccount(addr common.Address) {
	s.inner.CreateAccount(addr)
}

func (s *hookedStateDB) CreateContract(addr common.Address) {
	s.inner.CreateContract(addr)
}

func (s *hookedStateDB) GetBalance(addr common.Address) *uint256.Int {
	bal := s.inner.GetBalance(addr)
	if s.hooks.OnBalanceRead != nil {
		s.hooks.OnBalanceRead(addr, bal.ToBig())
	}
	return bal
}

func (s *hookedStateDB) GetNonce(addr common.Address) uint64 {
	nonce := s.inner.GetNonce(addr)
	if s.hooks.OnNonceRead != nil {
		s.hooks.OnNonceRead(addr, nonce)
	}
	return nonce
}

func (s *hookedStateDB) GetCodeHash(addr common.Address) common.Hash {
	codeHash := s.inner.GetCodeHash(addr)
	if s.hooks.OnCodeHashRead != nil {
		s.hooks.OnCodeHashRead(addr, codeHash)
	}
	return codeHash
}

func (s *hookedStateDB) GetCode(addr common.Address) []byte {
	code := s.inner.GetCode(addr)
	if s.hooks.OnCodeRead != nil {
		s.hooks.OnCodeRead(addr, code)
	}
	return code
}

func (s *hookedStateDB) GetCodeSize(addr common.Address) int {
	size := s.inner.GetCodeSize(addr)
	if s.hooks.OnCodeSizeRead != nil {
		s.hooks.OnCodeSizeRead(addr, size)
	}
	return size
}

func (s *hookedStateDB) AddRefund(u uint64) {
	s.inner.AddRefund(u)
}

func (s *hookedStateDB) SubRefund(u uint64) {
	s.inner.SubRefund(u)
}

func (s *hookedStateDB) GetRefund() uint64 {
	return s.inner.GetRefund()
}

func (s *hookedStateDB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	return s.inner.GetCommittedState(addr, hash)
}

func (s *hookedStateDB) GetState(addr common.Address, hash common.Hash) common.Hash {
	val := s.inner.GetState(addr, hash)
	if s.hooks.OnStorageRead != nil {
		s.hooks.OnStorageRead(addr, hash, val)
	}
	return val
}

func (s *hookedStateDB) GetStorageRoot(addr common.Address) common.Hash {
	return s.inner.GetStorageRoot(addr)
}

func (s *hookedStateDB) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return s.inner.GetTransientState(addr, key)
}

func (s *hookedStateDB) SetTransientState(addr common.Address, key, value common.Hash) {
	s.inner.SetTransientState(addr, key, value)
}

func (s *hookedStateDB) HasSelfDestructed(addr common.Address) bool {
	return s.inner.HasSelfDestructed(addr)
}

func (s *hookedStateDB) Exist(addr common.Address) bool {
	return s.inner.Exist(addr)
}

func (s *hookedStateDB) Empty(addr common.Address) bool {
	return s.inner.Empty(addr)
}

func (s *hookedStateDB) AddressInAccessList(addr common.Address) bool {
	return s.inner.AddressInAccessList(addr)
}

func (s *hookedStateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return s.inner.SlotInAccessList(addr, slot)
}

func (s *hookedStateDB) AddAddressToAccessList(addr common.Address) {
	s.inner.AddAddressToAccessList(addr)
}

func (s *hookedStateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	s.inner.AddSlotToAccessList(addr, slot)
}

func (s *hookedStateDB) PointCache() *utils.PointCache {
	return s.inner.PointCache()
}

func (s *hookedStateDB) Prepare(rules params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	s.inner.Prepare(rules, sender, coinbase, dest, precompiles, txAccesses)
}

func (s *hookedStateDB) RevertToSnapshot(i int) {
	s.inner.RevertToSnapshot(i)
}

func (s *hookedStateDB) Snapshot() int {
	return s.inner.Snapshot()
}

func (s *hookedStateDB) AddPreimage(hash common.Hash, bytes []byte) {
	s.inner.AddPreimage(hash, bytes)
}

func (s *hookedStateDB) Witness() *stateless.Witness {
	return s.inner.Witness()
}

func (s *hookedStateDB) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	prev := s.inner.SubBalance(addr, amount, reason)
	if s.hooks.OnBalanceChange != nil && !amount.IsZero() {
		newBalance := new(uint256.Int).Sub(&prev, amount)
		s.hooks.OnBalanceChange(addr, prev.ToBig(), newBalance.ToBig(), reason)
	}
	return prev
}

func (s *hookedStateDB) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	prev := s.inner.AddBalance(addr, amount, reason)
	if s.hooks.OnBalanceChange != nil && !amount.IsZero() {
		newBalance := new(uint256.Int).Add(&prev, amount)
		s.hooks.OnBalanceChange(addr, prev.ToBig(), newBalance.ToBig(), reason)
	}
	return prev
}

func (s *hookedStateDB) SetNonce(address common.Address, nonce uint64) {
	s.inner.SetNonce(address, nonce)
	if s.hooks.OnNonceChange != nil {
		s.hooks.OnNonceChange(address, nonce-1, nonce)
	}
}

func (s *hookedStateDB) SetCode(address common.Address, code []byte) {
	s.inner.SetCode(address, code)
	if s.hooks.OnCodeChange != nil {
		s.hooks.OnCodeChange(address, types.EmptyCodeHash, nil, crypto.Keccak256Hash(code), code)
	}
}

func (s *hookedStateDB) SetState(address common.Address, key common.Hash, value common.Hash) common.Hash {
	prev := s.inner.SetState(address, key, value)
	if s.hooks.OnStorageChange != nil && prev != value {
		s.hooks.OnStorageChange(address, key, prev, value)
	}
	return prev
}

func (s *hookedStateDB) SelfDestruct(address common.Address) uint256.Int {
	var prevCode []byte
	var prevCodeHash common.Hash

	if s.hooks.OnCodeChange != nil {
		prevCode = s.inner.GetCode(address)
		prevCodeHash = s.inner.GetCodeHash(address)
	}

	prev := s.inner.SelfDestruct(address)

	if s.hooks.OnBalanceChange != nil && !prev.IsZero() {
		s.hooks.OnBalanceChange(address, prev.ToBig(), new(big.Int), tracing.BalanceDecreaseSelfdestruct)
	}

	if s.hooks.OnCodeChange != nil && len(prevCode) > 0 {
		s.hooks.OnCodeChange(address, prevCodeHash, prevCode, types.EmptyCodeHash, nil)
	}

	return prev
}

func (s *hookedStateDB) SelfDestruct6780(address common.Address) (uint256.Int, bool) {
	var prevCode []byte
	var prevCodeHash common.Hash

	if s.hooks.OnCodeChange != nil {
		prevCodeHash = s.inner.GetCodeHash(address)
		prevCode = s.inner.GetCode(address)
	}

	prev, changed := s.inner.SelfDestruct6780(address)

	if s.hooks.OnBalanceChange != nil && changed && !prev.IsZero() {
		s.hooks.OnBalanceChange(address, prev.ToBig(), new(big.Int), tracing.BalanceDecreaseSelfdestruct)
	}

	if s.hooks.OnCodeChange != nil && changed && len(prevCode) > 0 {
		s.hooks.OnCodeChange(address, prevCodeHash, prevCode, types.EmptyCodeHash, nil)
	}

	return prev, changed
}

func (s *hookedStateDB) AddLog(log *types.Log) {
	// The inner will modify the log (add fields), so invoke that first
	s.inner.AddLog(log)
	if s.hooks.OnLog != nil {
		s.hooks.OnLog(log)
	}
}

func (s *hookedStateDB) Finalise(deleteEmptyObjects bool) {
	defer s.inner.Finalise(deleteEmptyObjects)
	if s.hooks.OnBalanceChange == nil {
		return
	}
	for addr := range s.inner.journal.dirties {
		obj := s.inner.stateObjects[addr]
		if obj != nil && obj.selfDestructed {
			// If ether was sent to account post-selfdestruct it is burnt.
			if bal := obj.Balance(); bal.Sign() != 0 {
				s.hooks.OnBalanceChange(addr, bal.ToBig(), new(big.Int), tracing.BalanceDecreaseSelfdestructBurn)
			}
		}
	}
}

// hookedReader wraps a Reader and invokes hooks when accounts and storage are loaded
type hookedReader struct {
	inner Reader
	hooks *tracing.Hooks
}

// newHookedReader creates a new hookedReader that wraps the given reader
func newHookedReader(reader Reader, hooks *tracing.Hooks) *hookedReader {
	return &hookedReader{
		inner: reader,
		hooks: hooks,
	}
}

// Account implements Reader, retrieving the account and invoking OnAccountLoad hook
func (r *hookedReader) Account(addr common.Address) (*types.StateAccount, error) {
	acct, err := r.inner.Account(addr)
	if err == nil && r.hooks.OnAccountLoad != nil {
		r.hooks.OnAccountLoad(addr, acct)
	}
	return acct, err
}

// Storage implements Reader, retrieving storage and invoking OnStorageLoad hook
func (r *hookedReader) Storage(addr common.Address, slot common.Hash) (common.Hash, error) {
	value, err := r.inner.Storage(addr, slot)
	if err == nil && r.hooks.OnStorageLoad != nil {
		r.hooks.OnStorageLoad(addr, slot, value)
	}
	return value, err
}

// Code implements Reader, retrieving the code associated with a particular account.
func (r *hookedReader) Code(addr common.Address, codeHash common.Hash) ([]byte, error) {
	code, err := r.inner.Code(addr, codeHash)
	if err == nil && r.hooks.OnCodeLoad != nil {
		r.hooks.OnCodeLoad(addr, code)
	}
	return code, err
}

// CodeSize implements Reader, returning the size of the code associated with a particular account.
func (r *hookedReader) CodeSize(addr common.Address, codeHash common.Hash) (int, error) {
	size, err := r.inner.CodeSize(addr, codeHash)
	if err != nil {
		return 0, err
	}
	code, err := r.inner.Code(addr, codeHash)
	if err == nil && r.hooks.OnCodeLoad != nil {
		r.hooks.OnCodeLoad(addr, code)
	}
	return size, err
}

// Copy implements Reader
func (r *hookedReader) Copy() Reader {
	return &hookedReader{
		inner: r.inner.Copy(),
		hooks: r.hooks,
	}
}
