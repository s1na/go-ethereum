package codetrie

import (
	"bytes"
	"encoding/hex"
	"fmt"
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
	Input  string
	Chunks []TChunk
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
		},
		{
			Input: strings.Repeat("6000", 15) + "00", // Len: 31
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 15) + "00",
				},
			},
		},
		{
			Input: strings.Repeat("6000", 16), // Len: 32
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 16),
				},
			},
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
		},
	}

	for _, c := range testCases {
		code, err := hex.DecodeString(c.Input)
		if err != nil {
			t.Error(err)
		}
		chunks := Chunkify(code, 32)
		if len(chunks) != len(c.Chunks) {
			t.Errorf("%v: invalid number of chunks: expected %d, got %d", t.Name(), len(c.Chunks), len(chunks))
		}
		for i, chunk := range chunks {
			expectedChunk := c.Chunks[i]
			fmt.Println(i, chunk)
			if chunk.fio != expectedChunk.fio {
				t.Errorf("%v: invalid chunk FIO: expected %d, got %d", t.Name(), expectedChunk.fio, chunk.fio)
			}
			expectedCode, err := hex.DecodeString(expectedChunk.code)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(chunk.code, expectedCode) {
				t.Errorf("%v: invalid chunk code: expected %s, got %s", t.Name(), expectedChunk.code, hex.EncodeToString(chunk.code))
			}
			db := trie.NewDatabase(memorydb.New())
			codeRoot, err := Merkleize(chunks, db)
			if err != nil {
				t.Error(err)
			}
			fmt.Printf("codeRoot: %v\n", codeRoot)
		}
	}
}
