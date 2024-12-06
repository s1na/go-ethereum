// Copyright 2023 The go-ethereum Authors
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

package eip7742

import (
	"testing"

	"github.com/ethereum/go-ethereum/params"
)

func TestCalcExcessBlobGas(t *testing.T) {
	var tests = []struct {
		excess uint64
		blobs  uint64
		want   uint64
	}{
		// The excess blob gas should not increase from zero if the used blob
		// slots are below - or equal - to the target.
		{0, 0, 0},
		{0, 1, 0},
		{0, params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob, 0},

		// If the target blob gas is exceeded, the excessBlobGas should increase
		// by however much it was overshot
		{0, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) + 1, params.BlobTxBlobGasPerBlob},
		{1, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) + 1, params.BlobTxBlobGasPerBlob + 1},
		{1, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) + 2, 2*params.BlobTxBlobGasPerBlob + 1},

		// The excess blob gas should decrease by however much the target was
		// under-shot, capped at zero.
		{params.BlobTxTargetBlobGasPerBlock, params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob, params.BlobTxTargetBlobGasPerBlock},
		{params.BlobTxTargetBlobGasPerBlock, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) - 1, params.BlobTxTargetBlobGasPerBlock - params.BlobTxBlobGasPerBlob},
		{params.BlobTxTargetBlobGasPerBlock, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) - 2, params.BlobTxTargetBlobGasPerBlock - (2 * params.BlobTxBlobGasPerBlob)},
		{params.BlobTxBlobGasPerBlob - 1, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) - 1, 0},
	}
	for i, tt := range tests {
		result := CalcExcessBlobGas(tt.excess, tt.blobs*params.BlobTxBlobGasPerBlob, params.BlobTxTargetBlobGasPerBlock/params.BlobTxBlobGasPerBlob)
		if result != tt.want {
			t.Errorf("test %d: excess blob gas mismatch: have %v, want %v", i, result, tt.want)
		}
	}
}
