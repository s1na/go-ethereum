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

package filtermaps

import (
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// registerMemoryReport hooks the filtermaps caches into the central
// memory reporter. The maps cache dominates the index's in-memory
// footprint and grows with the rendered log-index window.
func registerMemoryReport(f *FilterMaps) {
	memreport.Register("filtermaps/maps", func() memreport.Sample {
		var total uint64
		for _, k := range f.filterMapCache.Keys() {
			if m, ok := f.filterMapCache.Peek(k); ok {
				for _, row := range m {
					total += uint64(len(row)) * 4 // FilterRow is []uint32
				}
			}
		}
		return memreport.Sample{Bytes: total, Estimated: true}
	})
	memreport.Register("filtermaps/lvpointers", func() memreport.Sample {
		const perEntry = 32 // uint64 key + uint64 value + LRU node overhead
		return memreport.Sample{
			Bytes:     uint64(f.lvPointerCache.Len()) * perEntry,
			Estimated: true,
		}
	})
	memreport.Register("filtermaps/lastblocks", func() memreport.Sample {
		const perEntry = 64 // uint32 key + lastBlockOfMap struct + overhead
		return memreport.Sample{
			Bytes:     uint64(f.lastBlockCache.Len()) * perEntry,
			Estimated: true,
		}
	})
}
