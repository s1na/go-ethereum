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
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/triedb/pathdb"
)

// recentBlocksScanSlack is added on top of the pathdb diff-layer depth
// when sizing the canonical-block scan window. It absorbs the lag between
// the disk-layer flush and the indexer catching up.
const recentBlocksScanSlack = 64

// Sender-nonce lookup errors. These are surfaced through the JSON-RPC layer
// so clients can distinguish "unsupported backend" from "data outside the
// retained window" from "not in the pool, not yet mined".
var (
	// ErrSenderNonceLookupUnsupported is returned when the node is not
	// configured to answer historical sender-nonce queries (e.g., running
	// hashdb, or pathdb with state history disabled).
	ErrSenderNonceLookupUnsupported = errors.New("sender-nonce lookup requires pathdb with state history indexing")

	// ErrSenderNonceLookupUnavailable is returned when the target transaction
	// predates the retained state-history window, or when canonical block
	// bodies have been pruned out of reach.
	ErrSenderNonceLookupUnavailable = errors.New("state history does not cover this transaction (increase --history.state or run an archive node)")
)

// GetTransactionBySenderAndNonce returns the hash of the transaction with the
// given sender and nonce. The lookup proceeds in three tiers:
//
//  1. Transaction pool: if the tx is queued or pending, return its hash.
//  2. Latest state nonce: if the requested nonce is at or above the current
//     state nonce for the sender, the tx is not mined; return nil.
//  3. Pathdb state-history index: binary-search the sender's per-account
//     index for the block where the nonce transitioned from target to
//     target+1, then scan that block. If the binary search exhausts the
//     indexed history without finding such a transition (the answering
//     block is more recent than the indexer has reached, typically within
//     the diff-layer flush window), fall back to a bounded canonical-block
//     scan from the last indexed modification up to head.
//
// A nil hash with nil error indicates the (sender, nonce) has no matching
// tx (e.g., a contract whose nonce was bumped by CREATE rather than by a
// sent tx, or a genuinely unmined sender). A non-nil error surfaces a
// structured failure (unsupported backend, pruned window, indexer falling
// far behind).
func (b *EthAPIBackend) GetTransactionBySenderAndNonce(ctx context.Context, sender common.Address, nonce uint64) (*common.Hash, error) {
	// Tier 1: pool.
	if tx := b.eth.txPool.GetTxBySenderAndNonce(sender, nonce); tx != nil {
		h := tx.Hash()
		return &h, nil
	}

	// Tier 2: latest-state nonce gate. If the sender's current nonce is at
	// most the requested nonce, the tx has not been mined. The pool was
	// already checked, so return nil to mean "no such tx".
	latestState, _, err := b.StateAndHeaderByNumberOrHash(ctx, rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber))
	if err != nil {
		return nil, err
	}
	if latestState == nil {
		return nil, errors.New("latest state unavailable")
	}
	if nonce >= latestState.GetNonce(sender) {
		return nil, nil
	}

	// Tier 3: pathdb history index.
	pdb := b.eth.BlockChain().TrieDB().PathDB()
	if pdb == nil {
		return nil, ErrSenderNonceLookupUnsupported
	}

	idx, err := pdb.AccountHistoryIndex(sender)
	if err != nil {
		if errors.Is(err, pathdb.ErrStateHistoryNotIndexed) {
			return nil, ErrSenderNonceLookupUnsupported
		}
		return nil, err
	}
	n := idx.Count()

	// If the sender has no indexed modifications at all, their first ever
	// mod is either pre-sync (hashdb roots before the node started) or
	// inside the diff-layer flush window (very recently created sender).
	// Scan the canonical-head window to find it; surface unavailable on
	// miss because Tier 2 said the tx IS mined.
	if n == 0 {
		if h, err := b.scanRecentBlocks(sender, nonce); err != nil {
			return nil, err
		} else if h != nil {
			return h, nil
		}
		return nil, ErrSenderNonceLookupUnavailable
	}

	// Binary search. HistoricAccount returns the pre-state at the given
	// history id (the account state at the start of that block). The
	// pre-state nonce across the sender's indexed modifications is
	// monotonically non-decreasing. We want the index entry i-1 such that
	// pre[i-1] <= target and pre[i] > target; that entry's block is where
	// the sender's nonce transitioned past target.
	var probeErr error
	i := sort.Search(n, func(i int) bool {
		if probeErr != nil {
			return true
		}
		hid, err := idx.At(i)
		if err != nil {
			probeErr = err
			return true
		}
		acc, err := pdb.HistoricAccount(sender, hid)
		if err != nil {
			probeErr = err
			return true
		}
		if acc == nil {
			// Account did not exist yet at this history; treat as nonce 0.
			return false
		}
		return acc.Nonce > nonce
	})
	if probeErr != nil {
		return nil, probeErr
	}

	if i == 0 {
		// Even the earliest indexed entry has pre-state nonce > target, so
		// the answering tx predates the retained state-history window.
		return nil, ErrSenderNonceLookupUnavailable
	}

	if i < n {
		// Standard case: the transition is fully within indexed history.
		// Resolve and scan the block at position i-1.
		h, err := b.scanIndexedBlock(idx, i-1, sender, nonce)
		if err != nil {
			return nil, err
		}
		// A miss here is the contract-nonce-gap case: the sender's nonce
		// crossed target without an originating tx (e.g., contract
		// creation, or skipped nonces in legacy state). Report null.
		return h, nil
	}

	// i == n: all indexed pre-states <= target. The answering block is
	// either the latest indexed modification (when its post-state crosses
	// target) or a more recent block whose state history hasn't been
	// flushed to disk yet. Check the latest indexed block first, then scan
	// canonical blocks up to head.
	h, err := b.scanIndexedBlock(idx, n-1, sender, nonce)
	if err != nil {
		return nil, err
	}
	if h != nil {
		return h, nil
	}
	return b.scanRecentBlocks(sender, nonce)
}

// scanIndexedBlock resolves position `pos` in the sender's account history
// index to a canonical block and scans it for a matching (sender, nonce)
// tx. Returns nil hash, nil error on a miss.
func (b *EthAPIBackend) scanIndexedBlock(idx pathdb.HistoryIndexReader, pos int, sender common.Address, nonce uint64) (*common.Hash, error) {
	pdb := b.eth.BlockChain().TrieDB().PathDB()
	hid, err := idx.At(pos)
	if err != nil {
		return nil, err
	}
	blockNum, err := pdb.BlockNumberAt(hid)
	if err != nil {
		return nil, err
	}
	return b.findTxInBlock(blockNum, sender, nonce), nil
}

// scanRecentBlocks walks the most recent canonical blocks in reverse
// (head -> older) looking for a matching (sender, nonce) tx. This is the
// fallback for the unindexed tail of the state-history freezer: the
// pathdb diff-layer window plus a small slack for indexer lag.
//
// Reverse iteration matters because answering blocks tend to cluster
// near head (a sender whose tx falls into this branch typically just
// mined it); the common case terminates in a few block reads instead of
// sweeping the entire window.
//
// The window is sized from pathdb's diff-layer cap rather than a fixed
// constant so it tracks pathdb's actual configuration (defaults to 128;
// tests sometimes shrink it). The starting point is head-relative rather
// than sender-relative because a dormant sender's last indexed mod can
// be arbitrarily old but its unindexed mods can only live in this
// window.
func (b *EthAPIBackend) scanRecentBlocks(sender common.Address, nonce uint64) (*common.Hash, error) {
	head := b.eth.BlockChain().CurrentBlock().Number.Uint64()
	window := uint64(recentBlocksScanSlack)
	if pdb := b.eth.BlockChain().TrieDB().PathDB(); pdb != nil {
		window += uint64(pdb.MaxDiffLayers())
	}
	var lo uint64
	if head > window {
		lo = head - window
	}
	for bn := head; ; bn-- {
		if h := b.findTxInBlock(bn, sender, nonce); h != nil {
			return h, nil
		}
		if bn == lo {
			break
		}
	}
	return nil, nil
}

// findTxInBlock fetches the canonical block at blockNum and returns the
// hash of its first tx matching (sender, nonce), or nil on miss.
//
// Sender derivation is cheap for recently executed blocks because the tx
// objects carry a cached `from` after block execution; for deeper history
// the cache hit rate falls but only one or two probes typically reach this
// codepath per RPC call.
func (b *EthAPIBackend) findTxInBlock(blockNum uint64, sender common.Address, nonce uint64) *common.Hash {
	block := b.eth.BlockChain().GetBlockByNumber(blockNum)
	if block == nil {
		return nil
	}
	signer := types.MakeSigner(b.ChainConfig(), block.Number(), block.Time())
	for _, tx := range block.Transactions() {
		if tx.Nonce() != nonce {
			continue
		}
		s, err := types.Sender(signer, tx)
		if err != nil {
			continue
		}
		if s == sender {
			h := tx.Hash()
			return &h
		}
	}
	return nil
}
