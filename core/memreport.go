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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// withdrawalEstimateSize is the per-withdrawal RLP overhead estimate.
// Withdrawal is fixed-shape (index, validator, address, amount) and
// encodes to roughly this many bytes.
const withdrawalEstimateSize = 64

// registerChainMemoryReport wires the BlockChain's caches into the central
// memory reporter. Sizes are computed by iterating cached entries on each
// snapshot call, which is acceptable because snapshots happen at most once
// per second per consumer and cache sizes are small (256-2048 entries).
//
// All reported values are marked Estimated because the LRUs do not track
// bytes natively; we approximate by summing per-entry RLP sizes.
func registerChainMemoryReport(bc *BlockChain) {
	memreport.Register("chain/body", func() memreport.Sample {
		var total uint64
		for _, h := range bc.bodyCache.Keys() {
			if b, ok := bc.bodyCache.Peek(h); ok {
				total += bodyBytes(b)
			}
		}
		return memreport.Sample{Bytes: total, Estimated: true}
	})
	memreport.Register("chain/body-rlp", func() memreport.Sample {
		var total uint64
		for _, h := range bc.bodyRLPCache.Keys() {
			if b, ok := bc.bodyRLPCache.Peek(h); ok {
				total += uint64(len(b))
			}
		}
		return memreport.Sample{Bytes: total}
	})
	memreport.Register("chain/block", func() memreport.Sample {
		var total uint64
		for _, h := range bc.blockCache.Keys() {
			if b, ok := bc.blockCache.Peek(h); ok {
				total += b.Size()
			}
		}
		return memreport.Sample{Bytes: total, Estimated: true}
	})
	memreport.Register("chain/receipts", func() memreport.Sample {
		var total uint64
		for _, h := range bc.receiptsCache.Keys() {
			if rs, ok := bc.receiptsCache.Peek(h); ok {
				for _, r := range rs {
					total += uint64(r.Size())
				}
			}
		}
		return memreport.Sample{Bytes: total, Estimated: true}
	})
	memreport.Register("chain/tx-lookup", func() memreport.Sample {
		// Each entry: a hash key + LegacyTxLookupEntry (block hash + index)
		// plus pointer overhead. Most entries do not carry a transaction.
		const perEntry = 96
		return memreport.Sample{
			Bytes:     uint64(bc.txLookupCache.Len()) * perEntry,
			Estimated: true,
		}
	})
}

// registerHeaderMemoryReport wires the HeaderChain's caches.
func registerHeaderMemoryReport(hc *HeaderChain) {
	memreport.Register("chain/header", func() memreport.Sample {
		var total uint64
		for _, h := range hc.headerCache.Keys() {
			if hdr, ok := hc.headerCache.Peek(h); ok {
				total += uint64(hdr.Size())
			}
		}
		return memreport.Sample{Bytes: total, Estimated: true}
	})
	memreport.Register("chain/header-number", func() memreport.Sample {
		// Each entry: 32-byte hash key + 8-byte uint64 value + LRU overhead.
		const perEntry = 80
		return memreport.Sample{
			Bytes:     uint64(hc.numberCache.Len()) * perEntry,
			Estimated: true,
		}
	})
}

// bodyBytes approximates a Body's in-memory size by summing the sizes of
// its component slices.
func bodyBytes(b *types.Body) uint64 {
	var total uint64
	for _, tx := range b.Transactions {
		total += tx.Size()
	}
	for _, uncle := range b.Uncles {
		total += uint64(uncle.Size())
	}
	total += uint64(len(b.Withdrawals)) * withdrawalEstimateSize
	return total
}
