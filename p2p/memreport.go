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

package p2p

import (
	"github.com/ethereum/go-ethereum/internal/memreport"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// rlpxBufferBytes sums the read and write buffer capacities across all
// connected peers. The RLPx buffers grow to fit the largest frame seen
// on each connection and never shrink, so this number represents the
// process-wide network slack.
func (srv *Server) rlpxBufferBytes() uint64 {
	var total uint64
	srv.doPeerOp(func(peers map[enode.ID]*Peer) {
		for _, p := range peers {
			r, w := p.rw.bufferCapacity()
			if r > 0 {
				total += uint64(r)
			}
			if w > 0 {
				total += uint64(w)
			}
		}
	})
	return total
}

// registerMemoryReport hooks the server's network buffer accounting into
// the central memory reporter. Safe to call multiple times; later calls
// replace the prior closure.
func (srv *Server) registerMemoryReport() {
	memreport.Register("p2p/rlpx/buffers", func() memreport.Sample {
		return memreport.Sample{Bytes: srv.rlpxBufferBytes()}
	})
}
