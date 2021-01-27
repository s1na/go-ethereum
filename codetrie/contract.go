package codetrie

import (
	"errors"

	sszlib "github.com/ferranbt/fastssz"

	"github.com/ethereum/go-ethereum/rlp"
)

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
	if pc > len(c.code) {
		return errors.New("PC to touch exceeds bytecode length")
	}

	cid := pc / 32
	c.touchedChunks[cid] = true

	return nil
}

func (c *Contract) Prove() (*sszlib.CompressedMultiproof, error) {
	tree, err := GetSSZTree(c.code, 32)
	if err != nil {
		return nil, err
	}

	// ChunksLen and metadata fields
	mdIndices := []int{7, 8, 9, 10}
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

func serializeProof(p *sszlib.CompressedMultiproof) ([]byte, error) {
	return rlp.EncodeToBytes(p)
}
