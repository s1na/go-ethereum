package codetrie

import (
	"encoding/hex"
	"strings"
	"testing"
)

type ChunkifyTest struct {
	Input       string
	ExpectedNum int
}

func TestChunkifyNum(t *testing.T) {
	testCases := []ChunkifyTest{
		{
			Input:       "6000",
			ExpectedNum: 1,
		},
		{
			Input:       strings.Repeat("6000", 15) + "00", // Len: 31
			ExpectedNum: 1,
		},
		{
			Input:       strings.Repeat("6000", 16), // Len: 32
			ExpectedNum: 1,
		},
		{
			Input:       strings.Repeat("6000", 17), // Len: 32
			ExpectedNum: 2,
		},
	}

	for _, c := range testCases {
		code, err := hex.DecodeString(c.Input)
		if err != nil {
			t.Error(err)
		}
		chunks := Chunkify(code, 32)
		if len(chunks) != c.ExpectedNum {
			t.Errorf("%v: invalid number of chunks: expected %d, got %d", t.Name(), c.ExpectedNum, len(chunks))
		}
	}
}
