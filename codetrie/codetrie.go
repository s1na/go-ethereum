package codetrie

import (
	"math"
)

type Chunk struct {
	fio  uint8 // firstInstructionOffset
	code []byte
}

func NewChunk() *Chunk {
	return &Chunk{fio: 0, code: nil}
}

func Chunkify(code []byte, chunkSize uint) []*Chunk {
	numChunks := uint(math.Ceil(float64(len(code)) / float64(chunkSize)))
	chunks := make([]*Chunk, numChunks)
	return chunks
}
