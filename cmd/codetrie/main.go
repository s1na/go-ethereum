package main

import (
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/codetrie"
)

type ContractStat struct {
	CodeLen  int
	Code     string
	Duration int64
}

type Schema struct {
	Stats []ContractStat
}

func main() {
	f, err := ioutil.ReadFile("./contracts.json")
	if err != nil {
		log.Fatalf("Failed reading contracts file. Got error: %v\n", err)
	}

	var data Schema
	if err := json.Unmarshal(f, &data); err != nil {
		log.Fatalf("Failed unmarshalling json: %v\n", err)
	}

	type Record struct {
		codeLen   int
		durations []int64
	}
	res := make([]Record, len(data.Stats))

	runs := 10
	for i := 0; i < runs; i++ {
		for j, c := range data.Stats {
			codeHex := c.Code
			code, err := hex.DecodeString(codeHex)
			if err != nil {
				log.Fatalf("Failed decoding code hex: %v\n", err)
			}
			s := time.Now()
			codetrie.MerkleizeStack(code, 32)
			d := time.Since(s)
			res[j].codeLen = c.CodeLen
			res[j].durations = append(res[j].durations, d.Nanoseconds())
		}
	}

	cw := csv.NewWriter(os.Stdout)
	for _, item := range res {
		durationSum := int64(0)
		for i := 0; i < runs; i++ {
			durationSum += item.durations[i]
		}
		duration := durationSum / int64(runs)
		if err := cw.Write([]string{strconv.Itoa(item.codeLen), strconv.FormatInt(duration, 10)}); err != nil {
			log.Fatalf("error csv: %v\n", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Fatalf("after csv error: %v\n", err)
	}
}
