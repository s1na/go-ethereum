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

package pathdb

import (
	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// registerMemoryReport wires per-pool memory readers into memreport. The
// closures walk to the current bottom disk layer on every snapshot, so
// they stay correct across layer rebases and tree caps.
func registerMemoryReport(db *Database) {
	memreport.Register("triedb/pathdb/clean/nodes", func() memreport.Sample {
		return fastcacheBytes(db.tree.bottom().nodes)
	})
	memreport.Register("triedb/pathdb/clean/states", func() memreport.Sample {
		return fastcacheBytes(db.tree.bottom().states)
	})
	memreport.Register("triedb/pathdb/buffer", func() memreport.Sample {
		return memreport.Sample{Bytes: db.tree.bottom().buffer.size()}
	})
	memreport.Register("triedb/pathdb/diff", func() memreport.Sample {
		diffs, _ := db.Size()
		return memreport.Sample{Bytes: uint64(diffs)}
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
