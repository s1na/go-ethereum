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

package blobpool

import (
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// blobTxIndexEntrySize approximates per-transaction in-memory overhead
// in the blobpool: blobTxMeta plus map and slice headers. Blobs
// themselves are kept on disk in the billy store.
const blobTxIndexEntrySize = 200

// registerMemoryReport accounts for the blobpool's in-memory index. The
// pool keeps blob payloads on disk (billy) so memory is bounded by the
// number of indexed transactions.
func registerMemoryReport(p *BlobPool) {
	memreport.Register("txpool/blob", func() memreport.Sample {
		var count int
		p.lock.RLock()
		for _, txs := range p.index {
			count += len(txs)
		}
		p.lock.RUnlock()
		return memreport.Sample{
			Bytes:     uint64(count) * blobTxIndexEntrySize,
			Estimated: true,
		}
	})
}
