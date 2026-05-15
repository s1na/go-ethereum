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

package snap

import (
	"github.com/ethereum/go-ethereum/internal/memreport"
)

// inflightRequestCost is the per-request memory budget for an outstanding
// snap-sync request. Each in-flight request carries an account/storage/code
// task plus the response buffer; maxRequestSize bounds the response side.
const inflightRequestCost = maxRequestSize + 4096

// registerMemoryReport hooks the snap-sync request pipeline into the
// central memory reporter. We count outstanding requests across all
// snap-sync phases and multiply by maxRequestSize to estimate the peak
// in-flight network buffer footprint.
func registerMemoryReport(s *Syncer) {
	memreport.Register("eth/snap/inflight", func() memreport.Sample {
		s.lock.RLock()
		count := len(s.accountReqs) + len(s.storageReqs) + len(s.bytecodeReqs) +
			len(s.trienodeHealReqs) + len(s.bytecodeHealReqs)
		s.lock.RUnlock()
		return memreport.Sample{
			Bytes:     uint64(count) * inflightRequestCost,
			Estimated: true,
		}
	})
	memreport.Register("eth/snap/heal-pend", func() memreport.Sample {
		// trienodeHealPend is the count of trie nodes downloaded but not
		// yet processed. Each is roughly 600 bytes on average (mainnet).
		const perPendingNode = 600
		return memreport.Sample{
			Bytes:     s.trienodeHealPend.Load() * perPendingNode,
			Estimated: true,
		}
	})
}
