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

package rawdb

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
)

// makeCanonicalHeaders generates a continuous header chain [0, n] for testing.
func makeCanonicalHeaders(n int) []*types.Header {
	var (
		headers = make([]*types.Header, 0, n+1)
		parent  common.Hash
	)
	for i := 0; i <= n; i++ {
		header := &types.Header{
			ParentHash: parent,
			Number:     big.NewInt(int64(i)),
			Difficulty: big.NewInt(1),
			Extra:      []byte("test header"),
		}
		headers = append(headers, header)
		parent = header.Hash()
	}
	return headers
}

// newFreezerTestChain creates a freezer-backed database holding a fully
// populated canonical chain [0, n] in the key-value store.
func newFreezerTestChain(t *testing.T, n int) (ethdb.Database, []*types.Header) {
	t.Helper()

	db, err := Open(memorydb.New(), OpenOptions{Ancient: t.TempDir()})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	headers := makeCanonicalHeaders(n)
	for _, header := range headers {
		hash, number := header.Hash(), header.Number.Uint64()
		WriteHeader(db, header)
		WriteBody(db, hash, number, &types.Body{})
		WriteReceipts(db, hash, number, nil)
		WriteCanonicalHash(db, hash, number)
	}
	WriteHeadBlockHash(db, headers[n].Hash())
	return db, headers
}

// freezeAndCheck triggers a freeze cycle and verifies the resulting number of
// frozen items.
func freezeAndCheck(t *testing.T, db ethdb.Database, wantFrozen uint64) {
	t.Helper()

	if err := db.(*freezerdb).Freeze(); err != nil {
		t.Fatalf("failed to trigger freeze: %v", err)
	}
	frozen, err := db.Ancients()
	if err != nil {
		t.Fatalf("failed to read frozen item count: %v", err)
	}
	if frozen != wantFrozen {
		t.Fatalf("frozen item count mismatch: got %d, want %d", frozen, wantFrozen)
	}
}

// Tests that the chain freezer recovers a missing number-to-hash canonical
// mapping from the block data stored by hash, instead of stalling forever.
// Such holes can be left behind by an import that was interrupted between the
// block-data write and the head update (e.g. an unclean shutdown between
// newPayload and forkchoiceUpdated) and later skipped by snap sync.
func TestFreezeCanonicalHashRecovery(t *testing.T) {
	db, headers := newFreezerTestChain(t, 10)

	// Simulate the hole left by an interrupted import at height 6.
	DeleteCanonicalHash(db, 6)

	// Finalize block 8, making blocks [0, 8] eligible for freezing.
	WriteFinalizedBlockHash(db, headers[8].Hash())

	freezeAndCheck(t, db, 9)

	stored, err := db.Ancient(ChainFreezerHashTable, 6)
	if err != nil {
		t.Fatalf("failed to read frozen hash: %v", err)
	}
	if want := headers[6].Hash(); !bytes.Equal(stored, want[:]) {
		t.Fatalf("frozen hash mismatch: got %x, want %x", stored, want)
	}
}

// Tests that canonical mapping recovery picks the right block when multiple
// fully-populated siblings link to the same parent, by consulting the
// canonical child.
func TestFreezeCanonicalHashRecoveryAmbiguous(t *testing.T) {
	db, headers := newFreezerTestChain(t, 10)

	// Forge a fully-populated sibling of block 6, competing in recovery.
	sibling := &types.Header{
		ParentHash: headers[5].Hash(),
		Number:     big.NewInt(6),
		Difficulty: big.NewInt(1),
		Extra:      []byte("sibling"),
	}
	WriteHeader(db, sibling)
	WriteBody(db, sibling.Hash(), 6, &types.Body{})
	WriteReceipts(db, sibling.Hash(), 6, nil)

	DeleteCanonicalHash(db, 6)
	WriteFinalizedBlockHash(db, headers[8].Hash())

	freezeAndCheck(t, db, 9)

	stored, err := db.Ancient(ChainFreezerHashTable, 6)
	if err != nil {
		t.Fatalf("failed to read frozen hash: %v", err)
	}
	if want := headers[6].Hash(); !bytes.Equal(stored, want[:]) {
		t.Fatalf("frozen hash mismatch: got %x, want %x", stored, want)
	}
}

// Tests that the freezer still refuses to advance when the canonical mapping
// is missing and no fully-populated block is available for recovery, leaving
// the key-value store untouched.
func TestFreezeCanonicalHashUnrecoverable(t *testing.T) {
	db, headers := newFreezerTestChain(t, 10)

	// Wipe the canonical mapping and the receipts, making block 6 ineligible
	// for recovery.
	DeleteCanonicalHash(db, 6)
	DeleteReceipts(db, headers[6].Hash(), 6)
	WriteFinalizedBlockHash(db, headers[8].Hash())

	freezeAndCheck(t, db, 0)

	// The freeze cycle must have been aborted without wiping anything.
	if hash := ReadCanonicalHash(db, 7); hash != headers[7].Hash() {
		t.Fatalf("canonical mapping of block 7 was wiped despite aborted freeze")
	}
}
