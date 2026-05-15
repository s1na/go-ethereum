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

import "testing"

func TestRegisterAndSnapshot(t *testing.T) {
	h1 := Register("test/a", func() Sample { return Sample{Bytes: 1024} })
	h2 := Register("test/b", func() Sample { return Sample{Bytes: 2048, Estimated: true} })
	defer h1.Unregister()
	defer h2.Unregister()

	r := Snapshot()
	byName := map[string]Subsystem{}
	for _, s := range r.Subsystems {
		byName[s.Name] = s
	}
	if got := byName["test/a"]; got.Bytes != 1024 || got.Estimated {
		t.Errorf("test/a = %+v, want Bytes=1024 Estimated=false", got)
	}
	if got := byName["test/b"]; got.Bytes != 2048 || !got.Estimated {
		t.Errorf("test/b = %+v, want Bytes=2048 Estimated=true", got)
	}
	if r.TotalTracked < 1024+2048 {
		t.Errorf("TotalTracked = %d, want >= %d", r.TotalTracked, 1024+2048)
	}
}

func TestRegisterReplaces(t *testing.T) {
	Register("test/replaced", func() Sample { return Sample{Bytes: 100} })
	h := Register("test/replaced", func() Sample { return Sample{Bytes: 200} })
	defer h.Unregister()
	for _, s := range Snapshot().Subsystems {
		if s.Name == "test/replaced" && s.Bytes != 200 {
			t.Errorf("expected replacement to win, got Bytes=%d", s.Bytes)
		}
	}
}

func TestSnapshotPanickingSource(t *testing.T) {
	h := Register("test/panic", func() Sample { panic("boom") })
	defer h.Unregister()
	r := Snapshot() // must not panic
	for _, s := range r.Subsystems {
		if s.Name == "test/panic" && s.Bytes != 0 {
			t.Errorf("panicking source should report 0 bytes, got %d", s.Bytes)
		}
	}
}

func TestSnapshotSplitsOnAndOffHeap(t *testing.T) {
	a := Register("test/on", func() Sample { return Sample{Bytes: 100} })
	b := Register("test/off", func() Sample { return Sample{Bytes: 200, OffHeap: true} })
	defer a.Unregister()
	defer b.Unregister()

	r := Snapshot()
	var onHeap, offHeap uint64
	for _, s := range r.Subsystems {
		switch s.Name {
		case "test/on":
			onHeap = s.Bytes
			if s.OffHeap {
				t.Errorf("test/on should be on-heap")
			}
		case "test/off":
			offHeap = s.Bytes
			if !s.OffHeap {
				t.Errorf("test/off should be off-heap")
			}
		}
	}
	if onHeap != 100 || offHeap != 200 {
		t.Errorf("got on=%d off=%d, want on=100 off=200", onHeap, offHeap)
	}
	if r.TotalOnHeap < 100 {
		t.Errorf("TotalOnHeap = %d, want >= 100", r.TotalOnHeap)
	}
	if r.TotalOffHeap < 200 {
		t.Errorf("TotalOffHeap = %d, want >= 200", r.TotalOffHeap)
	}
}

func TestUnregisterIdempotent(t *testing.T) {
	h := Register("test/idem", func() Sample { return Sample{Bytes: 1} })
	h.Unregister()
	h.Unregister()
}
