// Copyright 2026 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

// TestCheckChainAllGood verifies that a freshly populated, consistent DB
// produces only info-level findings and exits with code 0.
func TestCheckChainAllGood(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	// Lay down a one-block canonical chain.
	header := &types.Header{Number: common.Big1}
	rawdb.WriteHeader(db, header)
	rawdb.WriteCanonicalHash(db, header.Hash(), 1)
	rawdb.WriteHeadHeaderHash(db, header.Hash())
	rawdb.WriteHeadBlockHash(db, header.Hash())
	rawdb.WriteHeadFastBlockHash(db, header.Hash())

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkHeadPointers(db, rep)
	checkUncleanShutdown(db, rep)

	if got := rep.exitCode(); got != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", got, buf.String())
	}
}

// TestCheckChainDanglingHead verifies that a head pointer with no header
// raises an error finding.
func TestCheckChainDanglingHead(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	// Write a head-header-hash that has no corresponding header / number mapping.
	rawdb.WriteHeadHeaderHash(db, common.HexToHash("0xdeadbeef"))

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkHeadPointers(db, rep)

	if got := rep.exitCode(); got != 2 {
		t.Fatalf("expected exit 2 (error), got %d\noutput:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "no_number_mapping") {
		t.Fatalf("expected no_number_mapping finding, got:\n%s", buf.String())
	}
}

// TestCheckChainCanonicalMismatch verifies that a head pointer whose canonical
// hash diverges raises an error.
func TestCheckChainCanonicalMismatch(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	headerA := &types.Header{Number: common.Big1, Extra: []byte("a")}
	headerB := &types.Header{Number: common.Big1, Extra: []byte("b")}
	rawdb.WriteHeader(db, headerA)
	rawdb.WriteHeader(db, headerB)
	// Canonical points at A, head pointer points at B.
	rawdb.WriteCanonicalHash(db, headerA.Hash(), 1)
	rawdb.WriteHeadHeaderHash(db, headerB.Hash())

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkHeadPointers(db, rep)

	if got := rep.exitCode(); got != 2 {
		t.Fatalf("expected exit 2 (error), got %d\noutput:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "canonical_mismatch") {
		t.Fatalf("expected canonical_mismatch finding, got:\n%s", buf.String())
	}
}

// TestCheckChainUncleanShutdownMarker verifies that a recorded crash list
// produces a warn finding.
func TestCheckChainUncleanShutdownMarker(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	// Mirror the on-disk shape by pushing a marker via the existing accessor,
	// which writes via rlp.EncodeToBytes of rawdb.UncleanShutdowns. Push
	// without a matching Pop is exactly what a crashed run leaves behind.
	if _, _, err := rawdb.PushUncleanShutdownMarker(db); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkUncleanShutdown(db, rep)

	if got := rep.exitCode(); got != 1 {
		t.Fatalf("expected exit 1 (warn), got %d\noutput:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "marker_present") {
		t.Fatalf("expected marker_present finding, got:\n%s", buf.String())
	}
}

// TestCheckChainSkeletonMultiSubchain verifies that a skeleton state with
// more than one subchain raises a warn finding (the symptom we saw in Max's
// log: stale_tail_1 = the failing block).
func TestCheckChainSkeletonMultiSubchain(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	prog := skeletonProgressLite{
		Subchains: []*subchainLite{
			{Head: 25_073_365, Tail: 25_073_364, Next: common.HexToHash("0x1e7bfa")},
			{Head: 24_795_537, Tail: 24_795_376, Next: common.HexToHash("0x1e27c1")},
		},
	}
	// The on-disk format used by eth/downloader/skeleton.go's
	// saveSyncStatus is JSON, not RLP.
	enc, err := json.Marshal(prog)
	if err != nil {
		t.Fatal(err)
	}
	rawdb.WriteSkeletonSyncStatus(db, enc)

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkUncleanShutdown(db, rep)

	if got := rep.exitCode(); got != 1 {
		t.Fatalf("expected exit 1 (warn), got %d\noutput:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "skeleton_multi_subchain") {
		t.Fatalf("expected skeleton_multi_subchain finding, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "24795376") {
		t.Fatalf("expected the stale tail block number to appear in output:\n%s", buf.String())
	}
}

// newTestChainDB returns an in-memory KV store backed by an on-disk freezer
// at a temp dir, plus a small frozen chain of the requested length. Block at
// height `numBlocks` (the next-to-freeze position) is the "boundary" block
// whose components live in the KV store, not the freezer.
//
// The first frozen `numBlocks` are written to the freezer via
// WriteAncientBlocks; the kv-side boundary block at height `numBlocks` is
// written via WriteBlock + WriteCanonicalHash + WriteReceipts to mimic a
// normal, healthy DB.
func newTestChainDB(t *testing.T, numBlocks int) ethdb.Database {
	t.Helper()
	frdir := t.TempDir()
	db, err := rawdb.Open(rawdb.NewMemoryDatabase(), rawdb.OpenOptions{Ancient: frdir})
	if err != nil {
		t.Fatalf("open chain db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Freeze blocks 0..numBlocks-1.
	blocks := make([]*types.Block, numBlocks)
	receipts := make([]types.Receipts, numBlocks)
	for i := 0; i < numBlocks; i++ {
		h := &types.Header{
			Number:      big.NewInt(int64(i)),
			Extra:       []byte{byte(i)},
			UncleHash:   types.EmptyUncleHash,
			TxHash:      types.EmptyTxsHash,
			ReceiptHash: types.EmptyReceiptsHash,
		}
		blocks[i] = types.NewBlockWithHeader(h)
		receipts[i] = nil
	}
	if _, err := rawdb.WriteAncientBlocks(db, blocks, types.EncodeBlockReceiptLists(receipts)); err != nil {
		t.Fatalf("WriteAncientBlocks: %v", err)
	}

	// Write a healthy boundary block at height numBlocks into the KV store.
	boundary := &types.Header{
		Number:      big.NewInt(int64(numBlocks)),
		Extra:       []byte("boundary"),
		UncleHash:   types.EmptyUncleHash,
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
	}
	boundBlock := types.NewBlockWithHeader(boundary)
	rawdb.WriteBlock(db, boundBlock)
	rawdb.WriteCanonicalHash(db, boundBlock.Hash(), boundBlock.NumberU64())
	rawdb.WriteReceipts(db, boundBlock.Hash(), boundBlock.NumberU64(), nil)
	return db
}

// TestCheckChainFreezerBoundaryHealthy verifies a well-formed boundary
// produces only info findings.
func TestCheckChainFreezerBoundaryHealthy(t *testing.T) {
	db := newTestChainDB(t, 5)

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkFreezerBoundary(db, rep)

	if got := rep.exitCode(); got != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "boundary_ok") {
		t.Fatalf("expected boundary_ok finding, got:\n%s", buf.String())
	}
}

// TestCheckChainFreezerBoundaryMissingCanonicalHash mimics the production bug:
// frozen blocks are present, the next-to-freeze block has header/body in KV
// but no canonical-hash mapping. The freezer would fail with "canonical hash
// missing" — check-chain should report it.
func TestCheckChainFreezerBoundaryMissingCanonicalHash(t *testing.T) {
	db := newTestChainDB(t, 5)
	// frozen = 5; corrupt by deleting the boundary canonical hash.
	rawdb.DeleteCanonicalHash(db, 5)

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkFreezerBoundary(db, rep)

	if got := rep.exitCode(); got != 2 {
		t.Fatalf("expected exit 2 (error), got %d\noutput:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "kv_canonical_hash_missing") {
		t.Fatalf("expected kv_canonical_hash_missing finding, got:\n%s", buf.String())
	}
}

// TestCheckChainFreezerBoundaryEmptyReceipts mimics the PR #31952 pattern:
// the boundary block has header + body + a zero-length value at the receipts
// key. The freezer trips on "block receipts missing". check-chain reports it.
func TestCheckChainFreezerBoundaryEmptyReceipts(t *testing.T) {
	db := newTestChainDB(t, 5)
	// Overwrite the receipts key with zero-length bytes (mimics the broken
	// snap-sync writeLive path before PR #31952).
	hash := rawdb.ReadCanonicalHash(db, 5)
	if hash == (common.Hash{}) {
		t.Fatal("expected canonical hash at boundary")
	}
	// Use WriteRawReceipts with a nil slice to simulate the empty-bytes write.
	rawdb.WriteRawReceipts(db, hash, 5, nil)

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkFreezerBoundary(db, rep)

	if got := rep.exitCode(); got != 2 {
		t.Fatalf("expected exit 2 (error), got %d\noutput:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "kv_receipts_missing") {
		t.Fatalf("expected kv_receipts_missing finding, got:\n%s", buf.String())
	}
}

// TestCheckChainFreezerBoundaryCaughtUp verifies that when the freezer has
// caught up to the chain head (frozen > head, i.e. there is no block at
// `frozen` yet), the KV-side probe is skipped instead of false-positiving.
func TestCheckChainFreezerBoundaryCaughtUp(t *testing.T) {
	db := newTestChainDB(t, 5)
	// frozen = 5; head pointer says the chain top is at the last frozen
	// block (i.e. frozen-1 == head), so there should not be a block at
	// frozen in the KV.
	headHash, err := db.Ancient(rawdb.ChainFreezerHashTable, 4)
	if err != nil {
		t.Fatalf("read frozen head hash: %v", err)
	}
	// Remove the synthetic boundary block written by newTestChainDB and
	// repoint head to the last-frozen block to mimic the "caught up" state.
	boundaryHash := rawdb.ReadCanonicalHash(db, 5)
	rawdb.DeleteCanonicalHash(db, 5)
	rawdb.DeleteHeader(db, boundaryHash, 5)
	rawdb.DeleteBody(db, boundaryHash, 5)
	rawdb.DeleteReceipts(db, boundaryHash, 5)
	rawdb.WriteHeadHeaderHash(db, common.BytesToHash(headHash))
	rawdb.WriteHeaderNumber(db, common.BytesToHash(headHash), 4)

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkFreezerBoundary(db, rep)

	if got := rep.exitCode(); got != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "freezer/caught_up") {
		t.Fatalf("expected freezer/caught_up finding, got:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "kv_canonical_hash_missing") {
		t.Fatalf("caught-up state must not fire kv_canonical_hash_missing:\n%s", buf.String())
	}
}

// TestCheckChainFreezerBoundaryNoFrozen verifies that a DB with frozen=0
// emits the empty-freezer info finding and exits 0.
func TestCheckChainFreezerBoundaryNoFrozen(t *testing.T) {
	db := rawdb.NewMemoryDatabase() // no freezer, Ancients() == 0

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	checkFreezerBoundary(db, rep)

	if got := rep.exitCode(); got != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", got, buf.String())
	}
	// A bare memory db reports the freezer as absent (no ancient store).
	if !strings.Contains(buf.String(), "freezer/absent") {
		t.Fatalf("expected freezer/absent finding, got:\n%s", buf.String())
	}
}

// TestCheckChainJSONMode verifies that JSON mode emits one valid JSON object
// per line with the expected envelope.
func TestCheckChainJSONMode(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	header := &types.Header{Number: common.Big1}
	rawdb.WriteHeader(db, header)
	rawdb.WriteCanonicalHash(db, header.Hash(), 1)
	rawdb.WriteHeadHeaderHash(db, header.Hash())

	var buf bytes.Buffer
	rep := newReporter(&buf, true)
	checkHeadPointers(db, rep)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one finding line")
	}
	for _, line := range lines {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		for _, key := range []string{"severity", "section", "type"} {
			if _, ok := v[key]; !ok {
				t.Fatalf("missing %q in finding: %s", key, line)
			}
		}
	}
}

// TestCheckChainConfigReport verifies the config section reports values that
// are present in the DB and pulled from the resolved eth config.
func TestCheckChainConfigReport(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	// Write a database version so the config section has something to print.
	rawdb.WriteDatabaseVersion(db, 9)

	cfg := &gethConfig{}
	cfg.Eth.NoPruning = true // -> gcmode=archive

	var buf bytes.Buffer
	rep := newReporter(&buf, false)
	reportConfig(db, cfg, rep)

	out := buf.String()
	for _, want := range []string{"database_version", "state_scheme", "gcmode", "archive", "history_mode"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in config output, got:\n%s", want, out)
		}
	}
}
