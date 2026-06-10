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

package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

// newSnapRepairTestChain sets up a chain holding blocks [1, 4] of a generated
// 8-block chain, imported via the snap-sync receipt-chain path. It returns the
// chain along with the full block range and encoded receipts, allowing tests
// to construct partially-written states for block 5.
func newSnapRepairTestChain(t *testing.T) (ethdb.Database, *BlockChain, types.Blocks, []rlp.RawValue) {
	t.Helper()

	gspec := &Genesis{
		Config:  params.TestChainConfig,
		BaseFee: big.NewInt(params.InitialBaseFee),
	}
	_, blocks, receipts := GenerateChainWithGenesis(gspec, ethash.NewFaker(), 8, func(i int, b *BlockGen) {
		b.SetCoinbase(common.Address{1})
	})
	enc := types.EncodeBlockReceiptLists(receipts)

	db := rawdb.NewMemoryDatabase()
	chain, err := NewBlockChain(db, gspec, ethash.NewFaker(), DefaultConfig().WithStateScheme(rawdb.HashScheme))
	if err != nil {
		t.Fatalf("failed to create chain: %v", err)
	}
	t.Cleanup(chain.Stop)

	if n, err := chain.InsertReceiptChain(blocks[:4], enc[:4], 0); err != nil {
		t.Fatalf("failed to insert receipt chain %d: %v", n, err)
	}
	return db, chain, blocks, enc
}

// Tests that the snap-sync receipt-chain insertion restores a missing
// canonical mapping of an already-known block, instead of skipping it and
// leaving a permanent hole that stalls the chain freezer. Such holes are left
// behind by an import that was interrupted between the block-data write and
// the head update (e.g. an unclean shutdown between newPayload and
// forkchoiceUpdated).
func TestInsertReceiptChainRepairsCanonicalHole(t *testing.T) {
	db, chain, blocks, enc := newSnapRepairTestChain(t)

	// Simulate the post-crash state for block 5: block data durable, head
	// update lost, so no number-to-hash mapping.
	block := blocks[4]
	batch := db.NewBatch()
	rawdb.WriteBlock(batch, block)
	rawdb.WriteRawReceipts(batch, block.Hash(), block.NumberU64(), enc[4])
	if err := batch.Write(); err != nil {
		t.Fatalf("failed to write block data: %v", err)
	}
	if rawdb.ReadCanonicalHash(db, 5) != (common.Hash{}) {
		t.Fatalf("test setup is broken: canonical mapping of block 5 should be absent")
	}
	// Snap sync over the damaged range must repair the mapping.
	if n, err := chain.InsertReceiptChain(blocks, enc, 0); err != nil {
		t.Fatalf("failed to insert receipt chain %d: %v", n, err)
	}
	if hash := rawdb.ReadCanonicalHash(db, 5); hash != block.Hash() {
		t.Fatalf("canonical mapping of block 5 was not repaired: got %x, want %x", hash, block.Hash())
	}
	if !rawdb.HasReceipts(db, blocks[7].Hash(), blocks[7].NumberU64()) {
		t.Fatalf("trailing blocks were not inserted")
	}
}

// Tests that the snap-sync receipt-chain insertion rewrites a canonical block
// whose receipts are missing, instead of skipping it based on the presence of
// the header and body alone.
func TestInsertReceiptChainRepairsReceiptHole(t *testing.T) {
	db, chain, blocks, enc := newSnapRepairTestChain(t)

	// Simulate a canonical block with the receipts missing.
	block := blocks[4]
	batch := db.NewBatch()
	rawdb.WriteBlock(batch, block)
	rawdb.WriteCanonicalHash(batch, block.Hash(), block.NumberU64())
	if err := batch.Write(); err != nil {
		t.Fatalf("failed to write block data: %v", err)
	}
	if rawdb.HasReceipts(db, block.Hash(), block.NumberU64()) {
		t.Fatalf("test setup is broken: receipts of block 5 should be absent")
	}
	// Snap sync over the damaged range must rewrite the block with receipts.
	if n, err := chain.InsertReceiptChain(blocks, enc, 0); err != nil {
		t.Fatalf("failed to insert receipt chain %d: %v", n, err)
	}
	if !rawdb.HasReceipts(db, block.Hash(), block.NumberU64()) {
		t.Fatalf("receipts of block 5 were not repaired")
	}
}
