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

package eth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/txpool/blobpool"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// TestGetTransactionBySenderAndNonce_Pool checks that a tx sitting in the
// pool is returned by hash without touching pathdb (Tier 1).
func TestGetTransactionBySenderAndNonce_Pool(t *testing.T) {
	b := initBackend(false)

	tx := makeTx(0, nil, nil, key)
	if err := b.SendTx(context.Background(), tx); err != nil {
		t.Fatalf("SendTx: %v", err)
	}

	got, err := b.GetTransactionBySenderAndNonce(context.Background(), address, 0)
	if err != nil {
		t.Fatalf("lookup error: %v", err)
	}
	if got == nil {
		t.Fatal("lookup returned nil for pooled tx")
	}
	if *got != tx.Hash() {
		t.Fatalf("hash mismatch: got %x, want %x", *got, tx.Hash())
	}
}

// TestGetTransactionBySenderAndNonce_NonceGate verifies Tier 2: when the
// requested nonce is at or above the sender's latest-state nonce and the
// pool does not contain such a tx, the result is (nil, nil) rather than an
// error.
func TestGetTransactionBySenderAndNonce_NonceGate(t *testing.T) {
	b := initBackend(false)

	// Sender's latest-state nonce is 0; nonce=5 has not been mined and is
	// not in the pool, so the answer is "no such tx", not an error.
	got, err := b.GetTransactionBySenderAndNonce(context.Background(), address, 5)
	if err != nil {
		t.Fatalf("lookup error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil hash, got %x", *got)
	}
}

// TestGetTransactionBySenderAndNonce_HashdbUnsupported verifies Tier 3 on
// hashdb-backed chains surfaces the documented unsupported error.
//
// Setup: generate a single block with a tx at nonce 0 from the funded
// account, import it. Now latest-state nonce is 1. Then look up nonce 0;
// the pool is empty (the tx was mined), nonce 0 < 1, so Tier 3 fires.
// Hashdb has no per-account state-history index, so the call must return
// ErrSenderNonceLookupUnsupported.
func TestGetTransactionBySenderAndNonce_HashdbUnsupported(t *testing.T) {
	engine := beacon.New(ethash.NewFaker())

	// Generate a 1-block chain with one tx from the funded address.
	var minedTx *types.Transaction
	_, blocks, _ := core.GenerateChainWithGenesis(gspec, engine, 1, func(i int, gen *core.BlockGen) {
		tx := makeTx(0, nil, nil, key)
		gen.AddTx(tx)
		minedTx = tx
	})

	options := &core.BlockChainConfig{
		TrieCleanLimit: 256,
		TrieDirtyLimit: 256,
		TrieTimeLimit:  5 * time.Minute,
		StateScheme:    rawdb.HashScheme,
		SnapshotLimit:  0,
	}
	chain, err := core.NewBlockChain(rawdb.NewMemoryDatabase(), gspec, engine, options)
	if err != nil {
		t.Fatalf("NewBlockChain: %v", err)
	}
	defer chain.Stop()
	if n, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("InsertChain block %d: %v", n, err)
	}

	txconfig := legacypool.DefaultConfig
	txconfig.Journal = ""
	blobPool := blobpool.New(blobpool.Config{Datadir: ""}, chain, nil)
	legacyPool := legacypool.New(txconfig, chain)
	pool, err := txpool.New(txconfig.PriceLimit, chain, []txpool.SubPool{legacyPool, blobPool})
	if err != nil {
		t.Fatalf("txpool.New: %v", err)
	}
	defer pool.Close()

	b := &EthAPIBackend{eth: &Ethereum{blockchain: chain, txPool: pool}}

	// Sanity: latest-state nonce should now be 1.
	st, _, err := b.StateAndHeaderByNumber(context.Background(), rpc.LatestBlockNumber)
	if err != nil {
		t.Fatalf("state lookup: %v", err)
	}
	if got := st.GetNonce(address); got != 1 {
		t.Fatalf("expected latest nonce 1 after mining, got %d", got)
	}

	// Tier 3 fires on hashdb: must return the documented sentinel.
	got, err := b.GetTransactionBySenderAndNonce(context.Background(), address, 0)
	if !errors.Is(err, ErrSenderNonceLookupUnsupported) {
		t.Fatalf("expected ErrSenderNonceLookupUnsupported, got err=%v, hash=%v", err, got)
	}
	_ = minedTx

	// Pool path still works: submit a tx at nonce 1 and verify lookup
	// returns its hash via Tier 1 (no Tier 3 involvement).
	tx2 := makeTx(1, nil, nil, key)
	if err := b.SendTx(context.Background(), tx2); err != nil {
		t.Fatalf("SendTx: %v", err)
	}
	gotHash, err := b.GetTransactionBySenderAndNonce(context.Background(), address, 1)
	if err != nil {
		t.Fatalf("Tier 1 lookup after mining: %v", err)
	}
	if gotHash == nil || *gotHash != tx2.Hash() {
		t.Fatalf("Tier 1 lookup mismatch: got %v, want %x", gotHash, tx2.Hash())
	}
}

// TestFindTxInBlock exercises the canonical-block tx-finding helper that
// underpins the recent-block scan fallback. The helper must locate a tx by
// (sender, nonce), tolerate nonce or signer mismatches, and return nil on
// no-match.
func TestFindTxInBlock(t *testing.T) {
	engine := beacon.New(ethash.NewFaker())

	var (
		minedNonce0 *types.Transaction
		minedNonce1 *types.Transaction
	)
	_, blocks, _ := core.GenerateChainWithGenesis(gspec, engine, 1, func(i int, gen *core.BlockGen) {
		minedNonce0 = makeTx(0, nil, nil, key)
		minedNonce1 = makeTx(1, nil, nil, key)
		gen.AddTx(minedNonce0)
		gen.AddTx(minedNonce1)
	})

	options := &core.BlockChainConfig{
		TrieCleanLimit: 256,
		TrieDirtyLimit: 256,
		TrieTimeLimit:  5 * time.Minute,
		StateScheme:    rawdb.HashScheme,
		SnapshotLimit:  0,
	}
	chain, err := core.NewBlockChain(rawdb.NewMemoryDatabase(), gspec, engine, options)
	if err != nil {
		t.Fatalf("NewBlockChain: %v", err)
	}
	defer chain.Stop()
	if n, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("InsertChain block %d: %v", n, err)
	}
	b := &EthAPIBackend{eth: &Ethereum{blockchain: chain}}
	const blockNum = 1

	// Hit: known sender, both nonces.
	if h := b.findTxInBlock(blockNum, address, 0); h == nil || *h != minedNonce0.Hash() {
		t.Fatalf("nonce=0 lookup: got %v, want %x", h, minedNonce0.Hash())
	}
	if h := b.findTxInBlock(blockNum, address, 1); h == nil || *h != minedNonce1.Hash() {
		t.Fatalf("nonce=1 lookup: got %v, want %x", h, minedNonce1.Hash())
	}
	// Miss: known sender, wrong nonce.
	if h := b.findTxInBlock(blockNum, address, 99); h != nil {
		t.Fatalf("nonce=99 lookup expected nil, got %x", *h)
	}
	// Miss: unknown sender, valid nonce.
	if h := b.findTxInBlock(blockNum, common.Address{0xAA}, 0); h != nil {
		t.Fatalf("unknown-sender lookup expected nil, got %x", *h)
	}
	// Miss: nonexistent block.
	if h := b.findTxInBlock(9999, address, 0); h != nil {
		t.Fatalf("nonexistent-block lookup expected nil, got %x", *h)
	}
}
