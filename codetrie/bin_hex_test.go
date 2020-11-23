package codetrie

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkBinVsHex(b *testing.B) {
	rand.Seed(time.Now().UnixNano())

	leaves := 8192
	keys := make([][]byte, leaves)
	vals := make([][]byte, leaves)
	for i := 0; i < leaves; i++ {
		key := make([]byte, 32)
		val := make([]byte, 32)
		rand.Read(key)
		rand.Read(val)
		keys[i] = key
		vals[i] = val
	}

	for i := 32; i <= 8192; i *= 2 {
		b.Run(fmt.Sprintf("bin-%d", i), func(b *testing.B) {
			BinWithLeaves(b, i, keys, vals)
		})
		b.Run(fmt.Sprintf("hex-%d", i), func(b *testing.B) {
			HexWithLeaves(b, i, keys, vals)
		})
	}
}

func BinWithLeaves(b *testing.B, num int, keys, vals [][]byte) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		trieB := trie.NewBinaryTrie()
		for j := 0; j < num; j++ {
			trieB.Update(keys[j], vals[j])
		}
		trieB.Hash()
	}
}

func HexWithLeaves(b *testing.B, num int, keys, vals [][]byte) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		trieH, err := trie.New(common.Hash{}, trie.NewDatabase(memorydb.New()))
		if err != nil {
			b.Fatalf("err: %v\n", err)
		}
		for j := 0; j < num; j++ {
			trieH.Update(keys[j], vals[j])
		}
		trieH.Hash()
	}
}
