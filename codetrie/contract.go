package codetrie

import (
	"errors"
	"sort"

	sszlib "github.com/ferranbt/fastssz"

	"github.com/ethereum/go-ethereum/codetrie/ssz"
	"github.com/ethereum/go-ethereum/common"
)

const CHUNK_SIZE = 32

type CMStats struct {
	NumContracts int
	ProofSize    int
	CodeSize     int
	ProofStats   *ssz.ProofStats
	RLPStats     *ssz.RLPStats
}

func NewCMStats() *CMStats {
	return &CMStats{
		ProofStats: &ssz.ProofStats{},
		RLPStats:   &ssz.RLPStats{},
	}
}

type ContractBag struct {
	contracts map[common.Hash]*Contract
	// TODO: remove
	LargeInitCodes map[common.Hash]int
}

func NewContractBag() *ContractBag {
	return &ContractBag{
		contracts:      make(map[common.Hash]*Contract),
		LargeInitCodes: make(map[common.Hash]int),
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

func (b *ContractBag) AddLargeInit(codeHash common.Hash, size int) {
	b.LargeInitCodes[codeHash] = size
}

func (b *ContractBag) Stats() (*CMStats, error) {
	stats := NewCMStats()
	stats.NumContracts = len(b.contracts)
	for _, c := range b.contracts {
		stats.CodeSize += c.CodeSize()
		rawProof, err := c.Prove()
		if err != nil {
			return nil, err
		}
		p := ssz.NewMultiproof(rawProof)
		cp := ssz.NewCompressedMultiproof(rawProof.Compress())

		ps := cp.ProofStats()
		stats.ProofStats.Add(ps)

		rs, err := ssz.NewRLPStats(p, cp)
		if err != nil {
			return nil, err
		}
		stats.RLPStats.Add(rs)
	}
	stats.ProofSize = stats.ProofStats.Sum()
	return stats, nil
}

type Contract struct {
	code          []byte
	touchedChunks map[int]bool
}

func NewContract(code []byte) *Contract {
	touchedChunks := make(map[int]bool)
	return &Contract{code: code, touchedChunks: touchedChunks}
}

func (c *Contract) TouchPC(pc int) error {
	if pc >= len(c.code) {
		return errors.New("PC to touch exceeds bytecode length")
	}

	cid := pc / CHUNK_SIZE
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

	fcid := from / CHUNK_SIZE
	tcid := to / CHUNK_SIZE
	for i := fcid; i < tcid+1; i++ {
		c.touchedChunks[i] = true
	}

	return nil
}

func (c *Contract) CodeSize() int {
	return len(c.code)
}

func (c *Contract) Prove() (*sszlib.Multiproof, error) {
	tree, err := GetSSZTree(c.code, CHUNK_SIZE)
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

	return p, nil
}

func (c *Contract) sortedTouchedChunks() []int {
	touched := make([]int, 0, len(c.touchedChunks))
	for k := range c.touchedChunks {
		touched = append(touched, k)
	}
	sort.Ints(touched)
	return touched
}
