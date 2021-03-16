package codetrie

import (
	"errors"
	"sort"

	sszlib "github.com/ferranbt/fastssz"
	"github.com/golang/snappy"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

type ContractBag struct {
	contracts map[common.Hash]*Contract
}

func NewContractBag() *ContractBag {
	return &ContractBag{
		contracts: make(map[common.Hash]*Contract),
	}
}

func (b *ContractBag) Get(codeHash common.Hash, code []byte) *Contract {
	if c, ok := b.contracts[codeHash]; ok {
		return c
	}

	c := NewContract(code)
	b.contracts[codeHash] = c
	return c
}

func (b *ContractBag) ProofSize() (int, error) {
	size := 0
	for _, v := range b.contracts {
		s, err := v.ProofSize()
		if err != nil {
			return 0, err
		}
		size += s
	}
	return size, nil
}

func (b *ContractBag) CodeSize() int {
	size := 0
	for _, v := range b.contracts {
		s := len(v.code)
		size += s
	}
	return size
}

type CMStats struct {
	NumContracts  int
	ProofSize     int
	CodeSize      int
	ProofStats    *ProofStats
	TouchedChunks []int
}

func (b *ContractBag) Stats() (*CMStats, error) {
	stats := &CMStats{
		NumContracts:  len(b.contracts),
		ProofStats:    &ProofStats{},
		TouchedChunks: make([]int, 0, len(b.contracts)),
	}
	for _, v := range b.contracts {
		stats.CodeSize += v.CodeSize()
		ps, err := v.ProofStats()
		if err != nil {
			return nil, err
		}
		stats.ProofStats.Add(ps)
		stats.TouchedChunks = append(stats.TouchedChunks, ps.TouchedChunks)
	}
	stats.ProofSize = stats.ProofStats.Sum()
	return stats, nil
}

type Contract struct {
	code          []byte
	chunks        []*Chunk
	touchedChunks map[int]bool
}

func NewContract(code []byte) *Contract {
	chunks := Chunkify(code, 32)
	touchedChunks := make(map[int]bool)
	return &Contract{code: code, chunks: chunks, touchedChunks: touchedChunks}
}

func (c *Contract) TouchPC(pc int) error {
	if pc >= len(c.code) {
		return errors.New("PC to touch exceeds bytecode length")
	}

	cid := pc / 32
	c.touchedChunks[cid] = true

	return nil
}

func (c *Contract) TouchRange(from, to int) error {
	if from >= to {
		return errors.New("Invalid range")
	}
	if to >= len(c.code) {
		return errors.New("PC to touch exceeds bytecode length")
	}

	fcid := from / 32
	tcid := to / 32
	for i := fcid; i < tcid+1; i++ {
		c.touchedChunks[i] = true
	}

	return nil
}

func (c *Contract) Prove() (*sszlib.CompressedMultiproof, error) {
	tree, err := GetSSZTree(c.code, 32)
	if err != nil {
		return nil, err
	}

	// ChunksLen and metadata fields
	mdIndices := []int{7, 8, 9, 10}

	touchedChunks := c.sortedTouchedChunks()
	chunkIndices := make([]int, 0, len(touchedChunks)*2)
	for k := range touchedChunks {
		// 6144 is global index for first chunk's node
		// Each chunk node has two children: FIO, code
		chunkIdx := 6144 + k
		chunkIndices = append(chunkIndices, chunkIdx*2)
		chunkIndices = append(chunkIndices, chunkIdx*2+1)
	}

	p, err := tree.ProveMulti(append(mdIndices, chunkIndices...))
	if err != nil {
		return nil, err
	}

	return p.Compress(), nil
}

func (c *Contract) ProofSize() (int, error) {
	p, err := c.Prove()
	if err != nil {
		return 0, err
	}

	size := 0
	// Interpret each index as a uint16
	size += len(p.Indices) * 2
	// 0 < level < 256, i.e. uint8
	size += len(p.ZeroLevels) * 1
	for _, v := range p.Hashes {
		size += len(v)
	}
	for _, v := range p.Leaves {
		size += len(v)
	}

	return size, nil
}

func (c *Contract) sortedTouchedChunks() []int {
	touched := make([]int, 0, len(c.touchedChunks))
	for k := range c.touchedChunks {
		touched = append(touched, k)
	}
	sort.Ints(touched)
	return touched
}

type ProofStats struct {
	RLPSize       int
	UnRLPSize     int
	SnappySize    int
	Indices       int
	ZeroLevels    int
	Hashes        int
	Leaves        int
	TouchedChunks int
}

func (ps *ProofStats) Add(o *ProofStats) {
	ps.RLPSize += o.RLPSize
	ps.UnRLPSize += o.UnRLPSize
	ps.SnappySize += o.SnappySize
	ps.Indices += o.Indices
	ps.ZeroLevels += o.ZeroLevels
	ps.Hashes += o.Hashes
	ps.Leaves += o.Leaves
	ps.TouchedChunks += o.TouchedChunks
}

func (ps *ProofStats) Sum() int {
	return ps.Indices + ps.ZeroLevels + ps.Hashes + ps.Leaves
}

func (c *Contract) ProofStats() (*ProofStats, error) {
	p, err := c.Prove()
	if err != nil {
		return nil, err
	}

	stats := &ProofStats{Indices: len(p.Indices) * 2, ZeroLevels: len(p.ZeroLevels) * 1, TouchedChunks: len(c.touchedChunks)}
	for _, v := range p.Hashes {
		stats.Hashes += len(v)
	}
	for i, v := range p.Leaves {
		in := p.Indices[i]
		// TODO: Hack as a temporary substitute for optimizing proof
		// to encode FIO as u8 instead of bytes32.
		if isIndexFIO(in) {
			stats.Leaves += 1
		} else {
			stats.Leaves += len(v)
		}
	}

	sp := getSerializableProof(p)
	rlpProof, err := serializeProof(sp)
	if err != nil {
		return nil, err
	}
	stats.RLPSize = len(rlpProof)

	// Measure snappy size of uncompressed proof
	dec := p.Decompress()
	unsp := getSerializableUnProof(dec)
	unrlpProof, err := serializeUnProof(unsp)
	if err != nil {
		return nil, err
	}
	compressedUnRLP := snappy.Encode(nil, unrlpProof)
	stats.SnappySize = len(compressedUnRLP)

	return stats, nil
}

func (c *Contract) CodeSize() int {
	return len(c.code)
}

type SerializableMultiproof struct {
	Indices    []uint16
	Leaves     [][]byte
	Hashes     [][]byte
	ZeroLevels []uint16
}

func copyElements(source []int) []uint16 {
	result := make([]uint16, len(source))
	for i, v := range source {
		result[i] = uint16(v)
	}
	return result
}

func getSerializableProof(p *sszlib.CompressedMultiproof) SerializableMultiproof {
	serializable := SerializableMultiproof{
		Indices:    make([]uint16, len(p.Indices)),
		ZeroLevels: make([]uint16, len(p.ZeroLevels)),
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

	serializable.Indices = copyElements(p.Indices)
	serializable.ZeroLevels = copyElements(p.ZeroLevels)

	return serializable

}

func serializeProof(s SerializableMultiproof) ([]byte, error) {
	serialized, err := rlp.EncodeToBytes(s)
	if err != nil {
		return nil, err
	}
	return serialized, err
}

// Serializable uncompressed multiproof
type SerializableUnMultiproof struct {
	Indices []uint16
	Leaves  [][]byte
	Hashes  [][]byte
}

func getSerializableUnProof(p *sszlib.Multiproof) SerializableUnMultiproof {
	serializable := SerializableUnMultiproof{
		Indices: make([]uint16, len(p.Indices)),
	}
	serializable.Hashes = p.Hashes
	serializable.Leaves = make([][]byte, len(p.Leaves))
	for i, v := range p.Leaves {
		serializable.Leaves[i] = v
	}

	serializable.Indices = copyElements(p.Indices)

	return serializable
}

func serializeUnProof(s SerializableUnMultiproof) ([]byte, error) {
	serialized, err := rlp.EncodeToBytes(s)
	if err != nil {
		return nil, err
	}
	return serialized, err
}

func isIndexFIO(i int) bool {
	return i >= 12288 && i%2 == 0
}
