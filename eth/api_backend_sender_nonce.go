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
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/triedb/pathdb"
)

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
//     target+1, then scan that block.
//
// A nil hash with nil error indicates the (sender, nonce) genuinely has no
// confirmed or queued tx. A non-nil error surfaces a structured failure
// (unsupported backend, pruned window, internal inconsistency).
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

	// Tier 3: pathdb history index. The sender has mined this nonce; locate
	// the containing block via binary search over the sender's account
	// history.
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
	if n == 0 {
		// The Tier 2 check already proved the tx was mined, so an empty
		// index means the answering history has been pruned out.
		return nil, ErrSenderNonceLookupUnavailable
	}

	// Binary search semantics: HistoricAccount returns the pre-state at the
	// given history id (the account state at the start of that block). The
	// pre-state nonce across the sender's indexed modifications is
	// monotonically non-decreasing. We want the index entry whose pre-state
	// nonce is the largest value <= target, then scan that block.
	//
	// Equivalently, sort.Search finds the smallest i such that
	// pre[i].Nonce > target, and the answering entry is i-1.
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
		// Even the earliest indexed entry already has pre-state nonce >
		// target, which means the answering tx predates the retained
		// state-history window.
		return nil, ErrSenderNonceLookupUnavailable
	}

	hid, err := idx.At(i - 1)
	if err != nil {
		return nil, err
	}
	blockNum, err := pdb.BlockNumberAt(hid)
	if err != nil {
		return nil, err
	}
	block := b.eth.BlockChain().GetBlockByNumber(blockNum)
	if block == nil {
		return nil, ErrSenderNonceLookupUnavailable
	}
	signer := types.MakeSigner(b.ChainConfig(), block.Number(), block.Time())
	for _, tx := range block.Transactions() {
		s, err := types.Sender(signer, tx)
		if err != nil {
			continue
		}
		if s == sender && tx.Nonce() == nonce {
			h := tx.Hash()
			return &h, nil
		}
	}
	// The index pointed at a block that does not contain a matching tx.
	// This indicates the index is inconsistent with the canonical chain.
	return nil, fmt.Errorf("sender-nonce index points at block %d, no matching tx for %x@%d", blockNum, sender, nonce)
}
