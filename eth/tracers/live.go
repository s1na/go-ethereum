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

package tracers

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	tracingV2 "github.com/ethereum/go-ethereum/core/tracing/v2"
)

// LiveDirectory is the collection of tracers which can be used
// during normal block import operations.
//
// Deprecated: It is left for backwards-compatibility with v1 tracers.
var LiveDirectory = liveDirectory{}

type liveDirectory struct{}

// Register registers a tracer constructor by name.
func (d *liveDirectory) Register(name string, f tracing.LiveConstructor) {
	tracingV2.LiveDirectory.Register(name, wrapV1(f))
}

func wrapV1(ctor tracing.LiveConstructor) tracingV2.NewLiveTracer {
	return func(config json.RawMessage) (*tracingV2.Hooks, error) {
		hooks, err := ctor(config)
		if err != nil {
			return nil, err
		}
		v2 := tracingV2.ToV2(hooks)
		v2.OnSystemCallStart = func(ctx *tracingV2.VMContext) {
			hooks.OnSystemCallStart()
		}
		v2.OnBalanceChange = func(addr common.Address, prev, new *big.Int, reason tracingV2.BalanceChangeReason) {
			hooks.OnBalanceChange(addr, prev, new, tracing.BalanceChangeReason(reason))
		}
		v2.OnGasChange = func(prev, new uint64, reason tracingV2.GasChangeReason) {
			hooks.OnGasChange(prev, new, tracing.GasChangeReason(reason))
		}
		return v2, nil
	}
}
