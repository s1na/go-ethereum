package codetrie

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
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
			CodeRoot: "fe1d3dcb57f6b06c53aaf7c6d33ae49f132472983798526b61e5a999fb37032d",
		},
		{
			Input: strings.Repeat("6000", 15) + "00", // Len: 31
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 15) + "00",
				},
			},
			CodeRoot: "79556343025dfe010078374be02ca81d2e2e37070bff7cf90870c779a4de579a",
		},
		{
			Input: strings.Repeat("6000", 16), // Len: 32
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 16),
				},
			},
			CodeRoot: "272a60435c819342d5e6003ea022d5777237f4a061e6cfa7887defc134939ca6",
		},
		{
			Input: strings.Repeat("6000", 17), // Len: 34
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
			CodeRoot: "87aa5ecc9c22f1342f3ce7fd534ed2cdad6c3b9cdf03616704c190aaa4d9cc4e",
		},
		{
			Input: strings.Repeat("58", 31) + "605b" + strings.Repeat("58", 30), // Len: 63
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
			CodeRoot: "a81b84e49034e54c8da52cc0c99423741ecae9bbfdedb159dfeef93be203b3cb",
		},
		{
			Input: strings.Repeat("58", 31) + "7f" + strings.Repeat("5b", 32) + strings.Repeat("58", 30), // Len: 94
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
			CodeRoot: "792f6e1cff9922e35e32b8db08807e9f8af007267565d6cadb4edec84c1fc300",
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

		expectedRoot, err := hex.DecodeString(c.CodeRoot)
		if err != nil {
			t.Error(err)
		}

		root, err := MerkleizeInMemory(code, 32)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(root.Bytes(), expectedRoot) {
			t.Errorf("%v: invalid code root: expected %s, got %s\n", t.Name(), c.CodeRoot, root.Hex())
		}

		// Test StackTrie impl
		stackRoot, err := MerkleizeStack(code, 32)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(stackRoot.Bytes(), expectedRoot) {
			t.Errorf("%v: invalid code root for MerkleizeStack: expected %s, got %s\n", t.Name(), c.CodeRoot, stackRoot.Hex())
		}
	}
}
