// Copyright 2024 The go-ethereum Authors
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

package native

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

func init() {
	tracers.DefaultDirectory.Register("blockHashTracer", newBlockHashTracer, false)
}

// blockHashTracer is a native go tracer that tracks blockhashes read by a transaction.
type blockHashTracer struct {
	blockhashes map[uint64]common.Hash
	reason      error
}

// newBlockHashTracer returns a new blockHashTracer.
func newBlockHashTracer(ctx *tracers.Context, _ json.RawMessage) (*tracers.Tracer, error) {
	t := &blockHashTracer{
		blockhashes: make(map[uint64]common.Hash),
		reason:      nil,
	}
	return &tracers.Tracer{
		Hooks: &tracing.Hooks{
			OnBlockHashRead: t.onBlockHashRead,
		},
		GetResult: t.getResult,
		Stop:      t.stop,
	}, nil
}

func (t *blockHashTracer) onBlockHashRead(blockNumber uint64, blockHash common.Hash) {
	t.blockhashes[blockNumber] = blockHash
}

// GetResult returns the json-encoded nested list of block hashes accessed by the transaction.
func (t *blockHashTracer) getResult() (json.RawMessage, error) {
	if t.reason != nil {
		return nil, t.reason
	}

	result := make(map[string]common.Hash)
	for blockNumber, blockHash := range t.blockhashes {
		result[new(big.Int).SetUint64(blockNumber).String()] = blockHash
	}

	return json.Marshal(result)
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *blockHashTracer) stop(err error) {
	t.reason = err
}
