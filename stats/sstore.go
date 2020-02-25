package stats

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
)

type statsColumn struct {
	name   string
	getter func(*SStoreStats) uint64
}

var columns = []statsColumn{
	{"Total", func(s *SStoreStats) uint64 { return s.total }},
	{"Init", func(s *SStoreStats) uint64 { return s.init }},
	{"Modify", func(s *SStoreStats) uint64 { return s.modify }},
	{"Clear", func(s *SStoreStats) uint64 { return s.clear }},
}

type SStoreStats struct {
	total  uint64
	init   uint64
	modify uint64
	clear  uint64
}

func NewSStoreStats() *SStoreStats {
	return &SStoreStats{}
}

func (s *SStoreStats) IncTotal(diff uint64) {
	atomic.AddUint64(&s.total, diff)
}

func (s *SStoreStats) IncInit(diff uint64) {
	atomic.AddUint64(&s.init, diff)
}

func (s *SStoreStats) DecInit(diff uint64) {
	atomic.AddUint64(&s.init, ^uint64(diff-1))
}

func (s *SStoreStats) IncModify(diff uint64) {
	atomic.AddUint64(&s.modify, diff)
}

func (s *SStoreStats) DecModify(diff uint64) {
	atomic.AddUint64(&s.modify, ^uint64(diff-1))
}

func (s *SStoreStats) IncClear(diff uint64) {
	atomic.AddUint64(&s.clear, diff)
}

func (s *SStoreStats) DecClear(diff uint64) {
	atomic.AddUint64(&s.clear, ^uint64(diff-1))
}

func (s *SStoreStats) Merge(o *SStoreStats) {
	atomic.AddUint64(&s.total, atomic.LoadUint64(&o.total))
	atomic.AddUint64(&s.init, atomic.LoadUint64(&o.init))
	atomic.AddUint64(&s.modify, atomic.LoadUint64(&o.modify))
	atomic.AddUint64(&s.clear, atomic.LoadUint64(&o.clear))
}

type StatsFile struct {
	file      io.WriteCloser
	buffer    *csv.Writer
	hasHeader bool
}

func NewStatsFile(path string) (*StatsFile, error) {
	_, err := os.Stat(path)
	appending := err == nil || !os.IsNotExist(err)

	w, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}

	return &StatsFile{file: w, buffer: csv.NewWriter(w), hasHeader: appending}, nil
}

func (s *StatsFile) writeHeader() error {
	header := make([]string, len(columns)+1)

	header[0] = "BlockNumber"
	for i, col := range columns {
		header[i+1] = col.name
	}

	return s.buffer.Write(header)
}

func (s *StatsFile) AddRow(blockNumber uint64, row *SStoreStats) error {
	if !s.hasHeader {
		log.Warn("Writing sstore stat headers")
		if err := s.writeHeader(); err != nil {
			return err
		}
		s.hasHeader = true
	}

	fields := make([]string, len(columns)+1)

	fields[0] = stringify(blockNumber)
	for i, col := range columns {
		fields[i+1] = stringify(col.getter(row))
	}

	return s.buffer.Write(fields)
}

func (s *StatsFile) Flush() error {
	s.buffer.Flush()
	return s.buffer.Error()
}

func (s *StatsFile) Close() error {
	s.buffer.Flush()
	if err := s.buffer.Error(); err != nil {
		return err
	}
	return s.file.Close()
}

func stringify(v uint64) string {
	return fmt.Sprintf("%d", v)
}
