package ssz

import (
	sszlib "github.com/ferranbt/fastssz"
	"github.com/golang/snappy"

	"github.com/ethereum/go-ethereum/rlp"
)

// Serializable uncompressed multiproof
type Multiproof struct {
	Indices []uint16
	Leaves  [][]byte
	Hashes  [][]byte
}

func NewMultiproof(p *sszlib.Multiproof) *Multiproof {
	serializable := &Multiproof{
		Indices: make([]uint16, len(p.Indices)),
	}

	serializable.Hashes = p.Hashes
	serializable.Leaves = make([][]byte, len(p.Leaves))
	for i, v := range p.Leaves {
		serializable.Leaves[i] = v
	}
	serializable.Indices = intSliceToUint16(p.Indices)

	return serializable
}

func (p *Multiproof) Serialize() ([]byte, error) {
	return rlp.EncodeToBytes(p)
}

// Serializable compressed multiproof
type CompressedMultiproof struct {
	Indices    []uint16
	Leaves     [][]byte
	Hashes     [][]byte
	ZeroLevels []uint8
}

func NewCompressedMultiproof(p *sszlib.CompressedMultiproof) *CompressedMultiproof {
	serializable := &CompressedMultiproof{
		Indices:    make([]uint16, len(p.Indices)),
		ZeroLevels: make([]uint8, len(p.ZeroLevels)),
	}
	serializable.Hashes = p.Hashes
	serializable.Leaves = make([][]byte, len(p.Leaves))
	for i, v := range p.Leaves {
		in := p.Indices[i]
		if isIndexFIO(in) {
			serializable.Leaves[i] = v[0:1]
		} else {
			serializable.Leaves[i] = v
		}
	}

	serializable.Indices = intSliceToUint16(p.Indices)
	serializable.ZeroLevels = intSliceToUint8(p.ZeroLevels)

	return serializable
}

func (p *CompressedMultiproof) Serialize() ([]byte, error) {
	return rlp.EncodeToBytes(p)
}

func (p *CompressedMultiproof) ProofStats() *ProofStats {
	stats := &ProofStats{Indices: len(p.Indices) * 2, ZeroLevels: len(p.ZeroLevels) * 1}
	for _, v := range p.Hashes {
		stats.Hashes += len(v)
	}
	for _, v := range p.Leaves {
		stats.Leaves += len(v)
	}
	return stats
}

type ProofStats struct {
	Indices    int
	ZeroLevels int
	Hashes     int
	Leaves     int
}

func (ps *ProofStats) Add(o *ProofStats) {
	ps.Indices += o.Indices
	ps.ZeroLevels += o.ZeroLevels
	ps.Hashes += o.Hashes
	ps.Leaves += o.Leaves
}

func (ps *ProofStats) Sum() int {
	return ps.Indices + ps.ZeroLevels + ps.Hashes + ps.Leaves
}

type RLPStats struct {
	RLPSize    int
	UnRLPSize  int
	SnappySize int
}

func NewRLPStats(p *Multiproof, cp *CompressedMultiproof) (*RLPStats, error) {
	stats := &RLPStats{}

	rlpProof, err := cp.Serialize()
	if err != nil {
		return nil, err
	}
	stats.RLPSize = len(rlpProof)

	// Measure snappy size of uncompressed proof
	unrlpProof, err := p.Serialize()
	if err != nil {
		return nil, err
	}
	stats.UnRLPSize = len(unrlpProof)
	compressedUnRLP := snappy.Encode(nil, unrlpProof)
	stats.SnappySize = len(compressedUnRLP)

	return stats, nil
}

func (rs *RLPStats) Add(o *RLPStats) {
	rs.RLPSize += o.RLPSize
	rs.UnRLPSize += o.UnRLPSize
	rs.SnappySize += o.SnappySize
}

func intSliceToUint16(source []int) []uint16 {
	result := make([]uint16, len(source))
	for i, v := range source {
		result[i] = uint16(v)
	}
	return result
}

func intSliceToUint8(source []int) []uint8 {
	result := make([]uint8, len(source))
	for i, v := range source {
		result[i] = uint8(v)
	}
	return result
}

func isIndexFIO(i int) bool {
	return i >= 12288 && i%2 == 0
}
