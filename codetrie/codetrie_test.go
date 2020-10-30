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
			CodeRoot: "aca110bb93e644288e180ceec9440cc5032088f7673ad0c37e68fb554589c87a",
		},
		{
			Input: strings.Repeat("6000", 15) + "00", // Len: 31
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 15) + "00",
				},
			},
			CodeRoot: "e77493ce728713f7ff736e9f85631ef03ff7960bace0f64e5f1a9cf312e1e46e",
		},
		{
			Input: strings.Repeat("6000", 16), // Len: 32
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 16),
				},
			},
			CodeRoot: "0a862d30de7d88dfa44b9036f677ca7d3d41f93ce8846ca3170d1f4a418a0b10",
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
			CodeRoot: "9f81e0ad1ebba8023d8ee6fc881a33a5a3e7540ef18363facba8211ad268c396",
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
			CodeRoot: "3cd6b50e5242f2ab73b05b28740e36a8d14501aa1d3118d084df6803ef71fa9f",
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
			CodeRoot: "3f814a80d8a465c466c961ae077b9e659451e1ce6b1164b8df314d3f454c664e",
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

		root, err := MerkleizeInMemory(code, 32)
		if err != nil {
			t.Error(err)
		}

		expectedRoot, err := hex.DecodeString(c.CodeRoot)
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(root.Bytes(), expectedRoot) {
			t.Errorf("%v: invalid code root: expected %s, got %s\n", t.Name(), c.CodeRoot, root.Hex())
		}
	}
}
