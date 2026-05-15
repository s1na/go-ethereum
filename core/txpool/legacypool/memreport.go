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

package legacypool

import (
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// registerMemoryReport wires the legacy txpool's slot occupancy into the
// central memory reporter. Each slot is up to txSlotSize bytes; the
// reported number is the configured slot capacity times the slot count,
// which is the worst-case in-memory footprint of the pool.
func registerMemoryReport(pool *LegacyPool) {
	memreport.Register("txpool/legacy", func() memreport.Sample {
		slots := pool.all.Slots()
		return memreport.Sample{
			Bytes:     uint64(slots) * uint64(txSlotSize),
			Estimated: true,
		}
	})
}
