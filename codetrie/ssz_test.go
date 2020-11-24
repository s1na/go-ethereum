package codetrie

import (
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	//"golang.org/x/crypto/sha3"
	"github.com/minio/sha256-simd"
)

func BenchmarkKeccakVsSha(b *testing.B) {
	nInputs := 1024
	inputs := make([][]byte, nInputs)
	for i := 0; i < nInputs; i++ {
		preimage := make([]byte, 64)
		rand.Read(preimage)
		inputs[i] = preimage
	}

	b.Run("sha256-simd", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			sha256.Sum256(inputs[0])
		}
	})

	b.Run("keccak256", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			crypto.Keccak256(inputs[0])
		}
	})
}
