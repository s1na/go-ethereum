package codetrie

import (
	"encoding/binary"
	"errors"
	"math"

	sszlib "github.com/ferranbt/fastssz"
	"golang.org/x/crypto/sha3"

	"github.com/ethereum/go-ethereum/codetrie/ssz"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
)

var (
	versionKey    = []byte{0xff, 0xfd}
	versionValue  = []byte{0x00}
	codeLengthKey = []byte{0xff, 0xfe}
	codeHashKey   = []byte{0xff, 0xff}
)

type Trie interface {
	Update(key, value []byte)
}

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
	trie, err := trie.New(common.Hash{}, db)
	if err != nil {
		return common.Hash{}, err
	}
	merkleize(code, chunkSize, trie)
	return trie.Hash(), nil
}

func MerkleizeStack(code []byte, chunkSize uint) (common.Hash, error) {
	trie := trie.NewStackTrie(memorydb.New())
	merkleize(code, chunkSize, trie)
	return trie.Hash(), nil
}

func MerkleizeBinary(code []byte, chunkSize uint) (common.Hash, error) {
	trie := trie.NewBinaryTrie()
	merkleize(code, chunkSize, trie)
	return common.BytesToHash(trie.Hash()), nil
}

func GetSSZTree(code []byte, chunkSize uint) (*sszlib.Node, error) {
	codeTrie, err := prepareSSZ(code, chunkSize)
	if err != nil {
		return nil, err
	}

	return codeTrie.GetTree()
}
func MerkleizeSSZSha(code []byte, chunkSize uint) (common.Hash, error) {
	codeTrie, err := prepareSSZ(code, chunkSize)
	if err != nil {
		return common.Hash{}, err
	}

	root, err := codeTrie.HashTreeRoot()
	if err != nil {
		return common.Hash{}, err
	}

	return common.BytesToHash(root[:]), nil
}

func MerkleizeSSZKeccak(code []byte, chunkSize uint) (common.Hash, error) {
	codeTrie, err := prepareSSZ(code, chunkSize)
	if err != nil {
		return common.Hash{}, err
	}

	hasher := sszlib.NewHasherWithHash(sha3.NewLegacyKeccak256())
	if err := codeTrie.HashTreeRootWith(hasher); err != nil {
		hasher.Reset()
		return common.Hash{}, err
	}

	root, err := hasher.HashRoot()
	hasher.Reset()

	return common.BytesToHash(root[:]), err
}

func prepareSSZ(code []byte, chunkSize uint) (*ssz.CodeTrie, error) {
	if chunkSize != 32 {
		return nil, errors.New("MerkleizeSSZ only supports chunk size of 32")
	}

	rawChunks := Chunkify(code, 32)
	chunks := make([]*ssz.Chunk, len(rawChunks))
	for i, rc := range rawChunks {
		code := rc.code
		if len(code) < 32 {
			code = make([]byte, 32)
			copy(code[:len(rc.code)], rc.code)
		}
		chunks[i] = &ssz.Chunk{FIO: rc.fio, Code: code}
	}

	metadata := &ssz.Metadata{Version: 0, CodeHash: crypto.Keccak256(code), CodeLength: uint16(len(code))}
	return &ssz.CodeTrie{Metadata: metadata, Chunks: chunks}, nil
}

func merkleize(code []byte, chunkSize uint, trie Trie) {
	chunks := Chunkify(code, chunkSize)
	merkleizeChunks(chunks, trie)

	// Insert metadata
	codeLen := BE(uint32(len(code)), 4)
	codeHash := crypto.Keccak256(code)

	trie.Update(versionKey, versionValue)
	trie.Update(codeLengthKey, codeLen)
	trie.Update(codeHashKey, codeHash)
}

func merkleizeChunks(chunks []*Chunk, trie Trie) {
	for i, chunk := range chunks {
		key := BE(uint16(i), 2)
		val := chunk.Serialize()
		trie.Update(key, val)
	}
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
			opcode := OpCode(op)
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
