package ssz

import (
	sszlib "github.com/ferranbt/fastssz"

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
