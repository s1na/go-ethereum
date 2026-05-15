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

package memreport

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// Lifecycle wraps the periodic logger so it can be registered with a
// node.Node via node.RegisterLifecycle.
type Lifecycle struct {
	Interval time.Duration

	stop func()
}

// Start spawns the periodic logger goroutine.
func (l *Lifecycle) Start() error {
	l.stop = LogLoop(l.Interval)
	return nil
}

// Stop terminates the periodic logger goroutine.
func (l *Lifecycle) Stop() error {
	if l.stop != nil {
		l.stop()
	}
	return nil
}

// LogLoop emits one INFO line per interval until the returned stop func
// is called. The line has the shape:
//
//	INFO Memory  rss=6.42GiB  heap=2.10GiB  trie/hashdb/clean=614MiB  ...
//
// Subsystems are listed in alphabetical order. Bytes are rendered with
// common.StorageSize for consistency with the rest of geth's logging.
func LogLoop(interval time.Duration) (stop func()) {
	if interval <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				logOnce()
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

func logOnce() {
	r := Snapshot()
	args := make([]interface{}, 0, 10+2*len(r.Subsystems))
	if r.Process.RSS > 0 {
		args = append(args, "rss", common.StorageSize(r.Process.RSS))
	}
	args = append(args,
		"heap", common.StorageSize(r.Runtime.HeapInUse),
		"on-heap", common.StorageSize(r.TotalOnHeap),
		"off-heap", common.StorageSize(r.TotalOffHeap),
	)
	if r.CacheBudget > 0 {
		args = append(args, "cache", common.StorageSize(r.CacheBudget))
		if r.Process.RSS > 0 {
			// Multiplier of observed RSS over the operator's --cache
			// setting. A value of 1.50x means the node is using 50%
			// more RAM than the configured cache budget; operators
			// use this to tune --cache down to fit a memory limit.
			args = append(args, "rss/cache",
				fmt.Sprintf("%.2fx", float64(r.Process.RSS)/float64(r.CacheBudget)))
		}
	}
	if r.Unaccounted > 0 {
		args = append(args, "unaccounted", common.StorageSize(r.Unaccounted))
	}
	for _, s := range r.Subsystems {
		if s.Bytes == 0 {
			continue
		}
		args = append(args, s.Name, common.StorageSize(s.Bytes))
	}
	log.Info("Memory", args...)
}
