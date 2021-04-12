package codetrie

import (
	"errors"
	"sort"

	sszlib "github.com/ferranbt/fastssz"

	"github.com/ethereum/go-ethereum/codetrie/ssz"
	"github.com/ethereum/go-ethereum/common"
)

var CHUNK_SIZES = [...]int{24, 32, 40}

type CMStats struct {
	NumContracts int
	ProofSizes   []int
	CodeSize     int
	ProofStats   []*ssz.ProofStats
	RLPStats     []*ssz.RLPStats
}

func NewCMStats() *CMStats {
	return &CMStats{
		ProofSizes: make([]int, len(CHUNK_SIZES)),
		ProofStats: make([]*ssz.ProofStats, len(CHUNK_SIZES)),
		RLPStats:   make([]*ssz.RLPStats, len(CHUNK_SIZES)),
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
		rawProofs, err := c.Prove()
		if err != nil {
			return nil, err
		}
		for i, rawProof := range rawProofs {
			p := ssz.NewMultiproof(rawProof)
			cp := ssz.NewCompressedMultiproof(rawProof.Compress())

			ps := cp.ProofStats()
			stats.ProofStats[i].Add(ps)

			rs, err := ssz.NewRLPStats(p, cp)
			if err != nil {
				return nil, err
			}
			stats.RLPStats[i].Add(rs)
		}
	}
	for i := 0; i < len(stats.ProofSizes); i++ {
		stats.ProofSizes[i] = stats.ProofStats[i].Sum()
	}
	return stats, nil
}

type Contract struct {
	code          []byte
	touchedChunks []map[int]bool
}

func NewContract(code []byte) *Contract {
	touchedChunks := make([]map[int]bool, len(CHUNK_SIZES))
	for i := 0; i < len(touchedChunks); i++ {
		touchedChunks[i] = make(map[int]bool)
	}
	return &Contract{code: code, touchedChunks: touchedChunks}
}

func (c *Contract) TouchPC(pc int) error {
	if pc >= len(c.code) {
		return errors.New("PC to touch exceeds bytecode length")
	}

	for i, s := range CHUNK_SIZES {
		cid := pc / s
		c.touchedChunks[i][cid] = true
	}

	return nil
}

func (c *Contract) TouchRange(from, to int) error {
	if from >= to {
		return errors.New("Invalid range")
	}
	if to >= len(c.code) {
		return errors.New("PC to touch exceeds bytecode length")
	}

	for i, s := range CHUNK_SIZES {
		fcid := from / s
		tcid := to / s
		for j := fcid; j < tcid+1; j++ {
			c.touchedChunks[i][j] = true
		}
	}

	return nil
}

func (c *Contract) CodeSize() int {
	return len(c.code)
}

func (c *Contract) Prove() ([]*sszlib.Multiproof, error) {
	proofs := make([]*sszlib.Multiproof, len(CHUNK_SIZES))
	for i, s := range CHUNK_SIZES {
		tree, err := GetSSZTree(c.code, uint(s))
		if err != nil {
			return nil, err
		}

		// ChunksLen and metadata fields
		mdIndices := []int{7, 8, 9, 10}

		touchedChunks := c.sortedTouchedChunks(i)
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

		proofs[i] = p
	}

	return proofs, nil
}

func (c *Contract) sortedTouchedChunks(sizeIndex int) []int {
	touched := make([]int, 0, len(c.touchedChunks[sizeIndex]))
	for k := range c.touchedChunks[sizeIndex] {
		touched = append(touched, k)
	}
	sort.Ints(touched)
	return touched
}
