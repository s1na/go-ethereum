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
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// knownHashBytes is the per-entry cost of an item in a knownCache: a
// 32-byte hash plus map overhead. mapset uses a hash-set under the
// hood; 48 bytes is a reasonable amortized estimate.
const knownHashBytes = 48

// registerHandlerMemoryReport hooks per-peer state aggregates into the
// central memory reporter. Each connected peer maintains separate
// tx-known and announce caches that we sum across all peers.
func registerHandlerMemoryReport(h *handler) {
	memreport.Register("eth/peers/known-txs", func() memreport.Sample {
		var total uint64
		h.peers.lock.RLock()
		for _, p := range h.peers.peers {
			total += uint64(p.KnownTxsLen()) * knownHashBytes
		}
		h.peers.lock.RUnlock()
		return memreport.Sample{Bytes: total, Estimated: true}
	})
}
