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
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestAccountHistoryIndex_NotIndexed(t *testing.T) {
	// Indexing disabled: every accessor should bail with ErrStateHistoryNotIndexed.
	env := newTester(t, &testerConfig{layers: 4, enableIndex: false})
	defer env.release()

	var addr common.Address
	for h := range env.accounts {
		addr = env.accountPreimage(h)
		break
	}

	if _, err := env.db.AccountHistoryIndex(addr); !errors.Is(err, ErrStateHistoryNotIndexed) {
		t.Fatalf("AccountHistoryIndex: want ErrStateHistoryNotIndexed, got %v", err)
	}
	if _, err := env.db.HistoricAccount(addr, 1); !errors.Is(err, ErrStateHistoryNotIndexed) {
		t.Fatalf("HistoricAccount: want ErrStateHistoryNotIndexed, got %v", err)
	}
}

func TestAccountHistoryIndex_Indexed(t *testing.T) {
	// Force diff-layer flushes so state history is actually written.
	maxDiffLayers = 4
	defer func() { maxDiffLayers = 128 }()

	env := newTester(t, &testerConfig{layers: 32, enableIndex: true})
	defer env.release()
	waitIndexing(env.db)

	// Pick any account whose index has at least one entry. The tester writes
	// random data per layer so most accounts will qualify, but we don't rely
	// on that. Scan until we find one.
	var (
		addr common.Address
		idx  HistoryIndexReader
	)
	for h := range env.accounts {
		a := env.accountPreimage(h)
		r, err := env.db.AccountHistoryIndex(a)
		if err != nil {
			t.Fatalf("AccountHistoryIndex(%x): %v", a, err)
		}
		if r.Count() > 0 {
			addr = a
			idx = r
			break
		}
	}
	if idx == nil {
		t.Fatal("no indexed account found across all current-state accounts")
	}

	// Every id reported by the index must yield a non-error HistoricAccount
	// read. The index promises an account record exists at every entry.
	for i := 0; i < idx.Count(); i++ {
		hid, err := idx.At(i)
		if err != nil {
			t.Fatalf("idx.At(%d): %v", i, err)
		}
		if _, err := env.db.HistoricAccount(addr, hid); err != nil {
			t.Fatalf("HistoricAccount(addr, %d): %v", hid, err)
		}
		blockNum, err := env.db.BlockNumberAt(hid)
		if err != nil {
			t.Fatalf("BlockNumberAt(%d): %v", hid, err)
		}
		if blockNum >= uint64(len(env.roots)) {
			t.Fatalf("BlockNumberAt(%d) = %d, beyond generated range %d", hid, blockNum, len(env.roots))
		}
	}

	tail, err := env.db.HistoryTail()
	if err != nil {
		t.Fatalf("HistoryTail: %v", err)
	}
	// Fresh db, no truncation: tail should be 0.
	if tail != 0 {
		t.Fatalf("HistoryTail: got %d, want 0", tail)
	}
}
