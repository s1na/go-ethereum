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

package filters

import (
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// logEntryOverhead approximates the per-Log fixed-shape bytes
// (Address, two hashes, four small ints, slice headers).
const logEntryOverhead = 144

// registerMemoryReport wires the logs cache into the central memory
// reporter. The cache is count-bounded (32 entries by default) but each
// entry can hold an arbitrary slice of logs, so worst-case size varies
// by orders of magnitude per block.
func registerMemoryReport(fs *FilterSystem) {
	memreport.Register("eth/filters/logs", func() memreport.Sample {
		var total uint64
		for _, h := range fs.logsCache.Keys() {
			e, ok := fs.logsCache.Peek(h)
			if !ok || e == nil {
				continue
			}
			for _, l := range e.logs {
				total += logEntryOverhead
				for _, t := range l.Topics {
					_ = t
					total += 32
				}
				total += uint64(len(l.Data))
			}
		}
		return memreport.Sample{Bytes: total, Estimated: true}
	})
}
