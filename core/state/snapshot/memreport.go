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

package snapshot

import (
	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// registerMemoryReport exposes the aggregated diff-layer memory and the
// disk-layer clean cache size to the central memory reporter.
func registerMemoryReport(t *Tree) {
	memreport.Register("snapshot/diff", func() memreport.Sample {
		diffs, _ := t.Size()
		return memreport.Sample{Bytes: uint64(diffs)}
	})
	memreport.Register("snapshot/clean", func() memreport.Sample {
		dl := t.disklayer()
		if dl == nil {
			return memreport.Sample{}
		}
		return fastcacheBytes(dl.cache)
	})
}

func fastcacheBytes(c *fastcache.Cache) memreport.Sample {
	if c == nil {
		return memreport.Sample{}
	}
	var stats fastcache.Stats
	c.UpdateStats(&stats)
	return memreport.Sample{Bytes: stats.BytesSize, OffHeap: true}
}
