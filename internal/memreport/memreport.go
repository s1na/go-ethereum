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

// Package memreport collects and reports per-subsystem memory occupancy.
//
// Subsystems register a Source at construction time; a Snapshot combines
// those numbers with Go runtime statistics and process RSS into a single
// structured report. The package owns no storage beyond the registration
// list; the Sources themselves read whatever live state the subsystem
// already maintains.
package memreport

import (
	"runtime/metrics"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Sample is the current memory occupancy reported by one Source.
type Sample struct {
	// Bytes is the size in bytes.
	Bytes uint64

	// Estimated marks a value derived by summing per-entry sizes from a
	// count-bounded cache or otherwise approximated, rather than read
	// directly from a byte counter the subsystem maintains.
	Estimated bool

	// OffHeap marks bytes allocated outside the Go heap (typically via
	// mmap or cgo). Off-heap bytes contribute to process RSS but are
	// invisible to runtime/metrics; the Snapshot machinery uses this
	// flag to separate them from on-heap totals when computing the
	// unaccounted residual.
	OffHeap bool
}

// Source returns the current Sample for one subsystem.
type Source func() Sample

// Handle controls a registered source. Call Unregister on shutdown so
// snapshots no longer call into a freed subsystem.
type Handle struct {
	name string
}

// Unregister removes the source. Idempotent.
func (h *Handle) Unregister() {
	if h == nil || h.name == "" {
		return
	}
	registry.remove(h.name)
	h.name = ""
}

// Register adds a memory source under name. Returns a Handle that can be
// used to remove it. If a source with the same name is already registered
// the new one replaces it; this keeps the API forgiving for repeated
// registrations during tests or subsystem rebuilds.
//
// Names are conventionally slash-separated, e.g. "triedb/hashdb/clean".
func Register(name string, fn Source) *Handle {
	registry.add(name, fn)
	return &Handle{name: name}
}

// Subsystem is one row in the report.
type Subsystem struct {
	Name      string `json:"name"`
	Bytes     uint64 `json:"bytes"`
	Estimated bool   `json:"estimated,omitempty"`
	OffHeap   bool   `json:"offHeap"`
}

// RuntimeStats are read from runtime/metrics each Snapshot.
type RuntimeStats struct {
	HeapAlloc    uint64 `json:"heapAlloc"`
	HeapInUse    uint64 `json:"heapInUse"`
	HeapIdle     uint64 `json:"heapIdle"`
	HeapSys      uint64 `json:"heapSys"`
	Stack        uint64 `json:"stack"`
	OtherClasses uint64 `json:"otherClasses"`
	Total        uint64 `json:"total"`
}

// ProcessStats hold OS-level numbers. Empty fields mean unavailable.
type ProcessStats struct {
	RSS   uint64 `json:"rss,omitempty"`
	VSize uint64 `json:"vsize,omitempty"`
}

// Report is a single snapshot of process memory state.
type Report struct {
	Time         time.Time    `json:"time"`
	Process      ProcessStats `json:"process"`
	Runtime      RuntimeStats `json:"runtime"`
	Subsystems   []Subsystem  `json:"subsystems"`
	TotalTracked uint64       `json:"totalTracked"`
	TotalOnHeap  uint64       `json:"totalOnHeap"`
	TotalOffHeap uint64       `json:"totalOffHeap"`
	CacheBudget  uint64       `json:"cacheBudget,omitempty"`
	Unaccounted  uint64       `json:"unaccounted,omitempty"`
}

// cacheBudget holds the --cache budget in bytes, as configured by the
// operator at startup. It is exposed in the Report and the periodic
// log line so operators can correlate the configured cache budget with
// observed RSS while tuning --cache.
var cacheBudget atomic.Uint64

// SetCacheBudget records the operator's --cache value (in bytes) so it
// can be reported alongside RSS. Pass 0 if no budget is known.
func SetCacheBudget(bytes uint64) {
	cacheBudget.Store(bytes)
}

// Snapshot returns a current Report.
//
// Unaccounted is computed as RSS minus the Go-managed memory classes
// (HeapSys+Stack+OtherClasses, from runtime/metrics) and the tracked
// off-heap subsystems. On-heap subsystems are not subtracted because
// their bytes are already inside HeapSys. The residual captures cgo
// allocations we have not instrumented, the binary's own code/data,
// shared library text, and fastcache freeChunks pool slack.
//
// Off-heap reads (notably fastcache and pebble's block cache) report
// the virtual size of allocated chunks, not the resident portion.
// Pages mmap'd but not yet touched are present in the virtual claim
// but absent from RSS. When the virtual off-heap claim exceeds the
// physical-memory residual, Unaccounted clamps to zero rather than
// reporting a negative number; this is a known limitation of measuring
// off-heap pools that allocate eagerly but page in lazily.
func Snapshot() Report {
	r := Report{
		Time:       time.Now(),
		Subsystems: registry.collect(),
		Runtime:    readRuntime(),
		Process:    readProcess(),
	}
	for _, s := range r.Subsystems {
		r.TotalTracked += s.Bytes
		if s.OffHeap {
			r.TotalOffHeap += s.Bytes
		} else {
			r.TotalOnHeap += s.Bytes
		}
	}
	r.CacheBudget = cacheBudget.Load()
	goManaged := r.Runtime.HeapSys + r.Runtime.Stack + r.Runtime.OtherClasses
	if r.Process.RSS > goManaged+r.TotalOffHeap {
		r.Unaccounted = r.Process.RSS - goManaged - r.TotalOffHeap
	}
	return r
}

// readRuntime reads the standard runtime/metrics memory classes.
func readRuntime() RuntimeStats {
	samples := []metrics.Sample{
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/memory/classes/heap/unused:bytes"},
		{Name: "/memory/classes/heap/free:bytes"},
		{Name: "/memory/classes/heap/released:bytes"},
		{Name: "/memory/classes/heap/stacks:bytes"},
		{Name: "/memory/classes/total:bytes"},
	}
	metrics.Read(samples)
	get := func(i int) uint64 {
		if samples[i].Value.Kind() == metrics.KindUint64 {
			return samples[i].Value.Uint64()
		}
		return 0
	}
	r := RuntimeStats{
		HeapAlloc: get(0),
		HeapIdle:  get(1) + get(2) + get(3),
		Stack:     get(4),
		Total:     get(5),
	}
	r.HeapInUse = r.HeapAlloc + get(1)
	r.HeapSys = r.HeapInUse + get(2) + get(3)
	if r.Total >= r.HeapSys+r.Stack {
		r.OtherClasses = r.Total - r.HeapSys - r.Stack
	}
	return r
}

// registry is the package-private source list.
var registry = &sourceRegistry{m: map[string]Source{}}

type sourceRegistry struct {
	mu sync.RWMutex
	m  map[string]Source
}

func (s *sourceRegistry) add(name string, fn Source) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[name] = fn
}

func (s *sourceRegistry) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, name)
}

func (s *sourceRegistry) collect() []Subsystem {
	s.mu.RLock()
	out := make([]Subsystem, 0, len(s.m))
	for name, fn := range s.m {
		sample := safeRead(name, fn)
		out = append(out, Subsystem{
			Name:      name,
			Bytes:     sample.Bytes,
			Estimated: sample.Estimated,
			OffHeap:   sample.OffHeap,
		})
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// safeRead invokes fn, swallowing panics so a buggy source cannot kill
// the snapshot path.
func safeRead(name string, fn Source) (s Sample) {
	defer func() {
		if r := recover(); r != nil {
			s = Sample{}
		}
	}()
	return fn()
}
