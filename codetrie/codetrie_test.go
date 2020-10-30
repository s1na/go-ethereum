package codetrie

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
)

type TChunk struct {
	fio  uint8
	code string
}

type ChunkifyTest struct {
	Input    string
	Chunks   []TChunk
	CodeRoot string
}

func TestChunkifyNum(t *testing.T) {
	testCases := []ChunkifyTest{
		{
			Input: "6000",
			Chunks: []TChunk{
				{
					fio:  0,
					code: "6000",
				},
			},
			CodeRoot: "ef9e4cfd85407737a19b6ebe65d8c0c5a408df123b893c86243e1f2f8b2e6571",
		},
		{
			Input: strings.Repeat("6000", 15) + "00", // Len: 31
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 15) + "00",
				},
			},
			CodeRoot: "e50ddf4f72c77fb3a0619e2b6a15de2b7f92e6cb7624393d70d8252ca9b93a9e",
		},
		{
			Input: strings.Repeat("6000", 16), // Len: 32
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 16),
				},
			},
			CodeRoot: "47a9bbdca825072b48978a7749c4a7b7080ddbda2e62ea9cd2d977d5adb0c7b8",
		},
		{
			Input: strings.Repeat("6000", 17), // Len: 32
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 16),
				},
				{
					fio:  0,
					code: "6000",
				},
			},
			CodeRoot: "e9c2c5da20557171a4109b4375e4156ae5b388b5b5ca61c737e7f054e7547aec",
		},
		{
			Input: strings.Repeat("58", 31) + "605b" + strings.Repeat("58", 30),
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 31) + "60",
				},
				{
					fio:  1,
					code: "5b" + strings.Repeat("58", 30),
				},
			},
			CodeRoot: "542d88bcafbd8a9a20ff945717ec2553f00afbfede842c38b262e5fd75f5914b",
		},
		{
			Input: strings.Repeat("58", 31) + "7f" + strings.Repeat("5b", 32) + strings.Repeat("58", 30),
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 31) + "7f",
				},
				{
					fio:  32,
					code: strings.Repeat("5b", 32),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 30),
				},
			},
			CodeRoot: "6e1ea988b9f21050644a147c8d69f0e9f78cd07e9966a69b21eada06ce9aec8d",
		},
	}

	for _, c := range testCases {
		code, err := hex.DecodeString(c.Input)
		if err != nil {
			t.Error(err)
		}
		chunks := Chunkify(code, 32)
		if len(chunks) != len(c.Chunks) {
			t.Errorf("%v: invalid number of chunks: expected %d, got %d\n", t.Name(), len(c.Chunks), len(chunks))
		}
		for i, chunk := range chunks {
			expectedChunk := c.Chunks[i]
			if chunk.fio != expectedChunk.fio {
				t.Errorf("%v: invalid chunk FIO: expected %d, got %d\n", t.Name(), expectedChunk.fio, chunk.fio)
			}
			expectedCode, err := hex.DecodeString(expectedChunk.code)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(chunk.code, expectedCode) {
				t.Errorf("%v: invalid chunk code: expected %s, got %s\n", t.Name(), expectedChunk.code, hex.EncodeToString(chunk.code))
			}
		}

		db := trie.NewDatabase(memorydb.New())
		codeTrie, err := MerkleizeChunks(chunks, db)
		if err != nil {
			t.Error(err)
		}

		expectedRoot, err := hex.DecodeString(c.CodeRoot)
		if err != nil {
			t.Error(err)
		}

		root := codeTrie.Hash()
		if !bytes.Equal(root.Bytes(), expectedRoot) {
			t.Errorf("%v: invalid code root: expected %s, got %s\n", t.Name(), c.CodeRoot, root.Hex())
		}

		// Check leaf values
		for i := 0; i < len(chunks); i++ {
			val, err := codeTrie.TryGet([]byte{0, byte(i)})
			if err != nil {
				t.Error(err)
			}
			expectedCode, err := hex.DecodeString(c.Chunks[i].code)
			if err != nil {
				t.Error(err)
			}
			expectedVal := append([]byte{c.Chunks[i].fio}, expectedCode...)
			if !bytes.Equal(val, expectedVal) {
				t.Errorf("%v: invalid trie leaf value: expected %v, got %v\n", t.Name(), expectedVal, val)
			}
		}
	}
}
