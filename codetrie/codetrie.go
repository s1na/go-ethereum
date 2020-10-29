package codetrie

import (
	"encoding/binary"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
)

type Chunk struct {
	fio  uint8 // firstInstructionOffset
	code []byte
}

func NewChunk() *Chunk {
	return &Chunk{fio: 0, code: nil}
}

func (c *Chunk) Serialize() []byte {
	return append([]byte{byte(c.fio)}, c.code...)
}

func MerkleizeInMemory(code []byte, chunkSize uint) (*trie.SecureTrie, error) {
	db := trie.NewDatabase(memorydb.New())
	return Merkleize(code, chunkSize, db)
}

func Merkleize(code []byte, chunkSize uint, db *trie.Database) (*trie.SecureTrie, error) {
	chunks := Chunkify(code, chunkSize)
	return MerkleizeChunks(chunks, db)
}

func MerkleizeChunks(chunks []*Chunk, db *trie.Database) (*trie.SecureTrie, error) {
	t, err := trie.NewSecure(common.Hash{}, db)
	if err != nil {
		return nil, err
	}

	for i, chunk := range chunks {
		key := encodeChunkKey(uint16(i))
		val := chunk.Serialize()
		t.Update(key, val)
	}

	return t, nil
}

func Chunkify(code []byte, chunkSize uint) []*Chunk {
	numChunks := uint(math.Ceil(float64(len(code)) / float64(chunkSize)))
	chunks := make([]*Chunk, numChunks)

	for i := uint(0); i < numChunks; i++ {
		startIdx := i * chunkSize
		endIdx := (i + 1) * chunkSize
		if i == numChunks-1 {
			endIdx = uint(len(code))
		}
		chunks[i] = &Chunk{fio: 0, code: code[startIdx:endIdx]}
	}

	setFIO(chunks)

	return chunks
}

func setFIO(chunks []*Chunk) {
	if len(chunks) < 2 {
		return
	}

	chunkSize := len(chunks[0].code)

	for i, chunk := range chunks {
		if i == len(chunks)-1 {
			break
		}

		for j, op := range chunk.code {
			opcode := vm.OpCode(op)
			if opcode.IsPush() {
				size := getPushSize(opcode)
				if j+size >= chunkSize {
					nextFIO := (j + size + 1) - chunkSize
					chunks[i+1].fio = uint8(nextFIO)
				}
			}
		}
	}
}

func getPushSize(opcode vm.OpCode) int {
	return (int(opcode) - 0x60) + 1
}

func encodeChunkKey(idx uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b[0:], idx)
	return b
}
