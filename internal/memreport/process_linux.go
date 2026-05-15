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

//go:build linux

package memreport

import (
	"os"
	"strconv"
	"strings"
)

// readProcess reads /proc/self/statm and returns RSS and VSize in bytes.
// Fields in statm are page counts, space-separated:
//
//	size resident shared text lib data dt
func readProcess() ProcessStats {
	raw, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return ProcessStats{}
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 2 {
		return ProcessStats{}
	}
	page := uint64(os.Getpagesize())
	vsize, _ := strconv.ParseUint(fields[0], 10, 64)
	rss, _ := strconv.ParseUint(fields[1], 10, 64)
	return ProcessStats{
		RSS:   rss * page,
		VSize: vsize * page,
	}
}
