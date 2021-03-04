package main

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/vm"
)

var hist map[vm.OpCode]int

func init() {
	hist = make(map[vm.OpCode]int)
}

func Step(op vm.OpCode) {
	if _, ok := hist[op]; !ok {
		hist[op] = 0
	}
	hist[op]++
}

func Result() (json.RawMessage, error) {
	return json.Marshal(hist)
}
