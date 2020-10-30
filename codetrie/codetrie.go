package codetrie

import (
	"encoding/binary"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
)

var (
	versionKey    = []byte{0xff, 0xfd}
	codeLengthKey = []byte{0xff, 0xfe}
	codeHashKey   = []byte{0xff, 0xff}

	versionValue = []byte{0x00}
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

func MerkleizeInMemory(code []byte, chunkSize uint) (common.Hash, error) {
	db := trie.NewDatabase(memorydb.New())
	return Merkleize(code, chunkSize, db)
}

func Merkleize(code []byte, chunkSize uint, db *trie.Database) (common.Hash, error) {
	chunks := Chunkify(code, chunkSize)
	trie, err := merkleizeChunks(chunks, db)
	if err != nil {
		return common.Hash{}, err
	}

	// Insert metadata
	trie.Update(versionKey, versionValue)
	codeLen := BE(uint32(len(code)), 4)
	trie.Update(codeLengthKey, codeLen)
	codeHash := crypto.Keccak256(code)
	trie.Update(codeHashKey, codeHash)

	return trie.Hash(), nil
}

func merkleizeChunks(chunks []*Chunk, db *trie.Database) (*trie.SecureTrie, error) {
	t, err := trie.NewSecure(common.Hash{}, db)
	if err != nil {
		return nil, err
	}

	for i, chunk := range chunks {
		key := BE(uint16(i), 2)
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

func BE(val interface{}, length int) []byte {
	var b []byte
	switch val.(type) {
	case uint16:
		b = make([]byte, 2)
		binary.BigEndian.PutUint16(b, val.(uint16))
	case uint32:
		b = make([]byte, 4)
		binary.BigEndian.PutUint32(b, val.(uint32))
	case uint64:
		b = make([]byte, 8)
		binary.BigEndian.PutUint64(b, val.(uint64))
	}
	return b
}
