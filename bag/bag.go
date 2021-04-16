package bag

import "github.com/ethereum/go-ethereum/common"

type Bag struct {
	LargeInitCodes map[common.Hash]int
}

func NewBag() *Bag {
	return &Bag{
		LargeInitCodes: make(map[common.Hash]int),
	}
}

func (b *Bag) AddLargeInit(codeHash common.Hash, size int) {
	b.LargeInitCodes[codeHash] = size
}
