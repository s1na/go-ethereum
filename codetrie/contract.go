package codetrie

import (
	"errors"

	sszlib "github.com/ferranbt/fastssz"

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
		s, err := v.ProofSize(false)
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
	ProofSize     int
	ProofSizeNoMD int
	CodeSize      int
}

func (b *ContractBag) Stats() (*CMStats, error) {
	stats := &CMStats{}
	for _, v := range b.contracts {
		stats.CodeSize += v.CodeSize()
		ps, err := v.ProofSize(false)
		if err != nil {
			return nil, err
		}
		stats.ProofSize += ps
		nm, err := v.ProofSize(true)
		if err != nil {
			return nil, err
		}
		stats.ProofSizeNoMD += nm
	}
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

func (c *Contract) Prove(noMD bool) (*sszlib.CompressedMultiproof, error) {
	tree, err := GetSSZTree(c.code, 32)
	if err != nil {
		return nil, err
	}

	// ChunksLen and metadata fields
	mdIndices := []int{}
	if !noMD {
		mdIndices = []int{7, 8, 9, 10}
	}
	chunkIndices := make([]int, 0, len(c.touchedChunks)*2)
	for k := range c.touchedChunks {
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

func (c *Contract) ProofSize(noMD bool) (int, error) {
	p, err := c.Prove(noMD)
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

func (c *Contract) CodeSize() int {
	return len(c.code)
}

func serializeProof(p *sszlib.CompressedMultiproof) ([]byte, error) {
	return rlp.EncodeToBytes(p)
}
