// Copyright 2026 The go-ethereum Authors
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

package pathdb

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// ErrStateHistoryNotIndexed is returned when the state history index is not
// available on this database, either because the indexer is disabled, the
// freezer is not configured, or the initial build has not yet finished.
var ErrStateHistoryNotIndexed = errors.New("state history is not indexed")

// ErrStateHistoryPruned is returned when the requested state history id has
// been pruned from the freezer tail.
var ErrStateHistoryPruned = errors.New("state history has been pruned")

// HistoryIndexReader provides random-access reads over the sorted list of
// state-history ids at which a single state element was modified. The
// sequence is strictly increasing.
type HistoryIndexReader interface {
	// Count returns the total number of indexed modifications.
	Count() int

	// At returns the i-th history id. The caller must ensure 0 <= i < Count().
	At(i int) (uint64, error)
}

// accountHistoryIndexReader exposes ordinal access over an account's history
// index. Not safe for concurrent use.
type accountHistoryIndexReader struct {
	reader *indexReader
}

// Count implements HistoryIndexReader.
func (r *accountHistoryIndexReader) Count() int {
	return r.reader.count()
}

// At implements HistoryIndexReader.
func (r *accountHistoryIndexReader) At(i int) (uint64, error) {
	return r.reader.at(i)
}

// AccountHistoryIndex returns a random-access reader over the state-history
// ids at which the given account was modified. The returned reader is not
// safe for concurrent use.
//
// Returns ErrStateHistoryNotIndexed if this database does not maintain a
// state-history index (indexer disabled, freezer absent, or initial build
// still in progress).
func (db *Database) AccountHistoryIndex(addr common.Address) (HistoryIndexReader, error) {
	if err := db.checkStateIndexerReady(); err != nil {
		return nil, err
	}
	ident := newAccountIdent(crypto.Keccak256Hash(addr.Bytes()))
	r, err := newIndexReader(db.diskdb, ident, 0)
	if err != nil {
		return nil, err
	}
	return &accountHistoryIndexReader{reader: r}, nil
}

// MaxDiffLayers returns the maximum number of in-memory diff layers the
// database keeps on top of the disk layer. Blocks within this many of
// canonical head have not yet been flushed to the state-history freezer
// and are therefore not represented in the per-account state-history
// index; callers that need to cover the unindexed tail (e.g., for
// sender-nonce lookup) should size their scan window from this value.
func (db *Database) MaxDiffLayers() int {
	return maxDiffLayers
}

// HistoryTail returns the smallest state-history id currently retained in the
// freezer. Ids less than or equal to this value have been pruned and are not
// queryable. Returns ErrStateHistoryNotIndexed if the freezer is unavailable.
func (db *Database) HistoryTail() (uint64, error) {
	if db.stateFreezer == nil {
		return 0, ErrStateHistoryNotIndexed
	}
	return db.stateFreezer.Tail()
}

// BlockNumberAt returns the block number associated with the given
// state-history id. Returns ErrStateHistoryPruned if the id falls at or below
// the current freezer tail.
func (db *Database) BlockNumberAt(historyID uint64) (uint64, error) {
	if db.stateFreezer == nil {
		return 0, ErrStateHistoryNotIndexed
	}
	tail, err := db.stateFreezer.Tail()
	if err != nil {
		return 0, err
	}
	if historyID <= tail {
		return 0, ErrStateHistoryPruned
	}
	m, err := readStateHistoryMeta(db.stateFreezer, historyID)
	if err != nil {
		return 0, err
	}
	return m.block, nil
}

// HistoricAccount returns the pre-state of addr at the given history id,
// i.e., the account state at the start of the block recorded by that
// history. Returns nil if the account did not exist immediately before the
// transition.
//
// The history id must correspond to an entry where addr was actually
// modified (typically one drawn from AccountHistoryIndex(addr)).
func (db *Database) HistoricAccount(addr common.Address, historyID uint64) (*types.SlimAccount, error) {
	blob, err := db.HistoricAccountRLP(addr, historyID)
	if err != nil {
		return nil, err
	}
	if len(blob) == 0 {
		return nil, nil
	}
	account := new(types.SlimAccount)
	if err := rlp.DecodeBytes(blob, account); err != nil {
		return nil, fmt.Errorf("failed to decode account: %w", err)
	}
	return account, nil
}

// HistoricAccountRLP returns the slim-RLP-encoded pre-state of addr at the
// given history id, i.e., the account state at the start of the block
// recorded by that history. Returns an empty slice if the account did not
// exist immediately before the transition.
//
// The history id must correspond to an entry where addr was actually
// modified (typically one drawn from AccountHistoryIndex(addr)). The
// indexer guarantees an account-data record exists at every such id.
func (db *Database) HistoricAccountRLP(addr common.Address, historyID uint64) ([]byte, error) {
	if err := db.checkStateIndexerReady(); err != nil {
		return nil, err
	}
	r := newStateHistoryReader(db.diskdb, db.stateFreezer)
	return r.readAccount(addr, historyID)
}

// checkStateIndexerReady returns nil if the state-history indexer is
// available and the initial build has completed.
func (db *Database) checkStateIndexerReady() error {
	if db.stateIndexer == nil || db.stateFreezer == nil {
		return ErrStateHistoryNotIndexed
	}
	if !db.stateIndexer.inited() {
		return ErrStateHistoryNotIndexed
	}
	return nil
}
