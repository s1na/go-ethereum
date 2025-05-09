package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/node"
)

var (
	datadir  = flag.String("datadir", "", "Path to the mainnet datadir")
	threads  = flag.Int("threads", 100, "Number of concurrent threads")
	duration = flag.Duration("duration", 10*time.Second, "Test duration")
)

func main() {
	flag.Parse()

	if *datadir == "" {
		log.Fatal("Please specify --datadir")
	}

	// Open the database
	stack, err := node.New(&node.Config{
		DataDir: *datadir,
	})
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}
	defer stack.Close()

	// Open the database
	db, err := stack.OpenDatabase("chaindata", 0, 0, "", false)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create channels for results
	results := make(chan time.Duration, *threads*1000)
	var wg sync.WaitGroup

	// Start the test
	start := time.Now()
	end := start.Add(*duration)

	// Launch worker goroutines
	for i := 0; i < *threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(end) {
				readStart := time.Now()
				rawdb.ReadTxIndexTail(db)
				results <- time.Since(readStart)
			}
		}()
	}

	// Start a goroutine to close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var (
		total     time.Duration
		count     int64
		min       = time.Hour
		max       time.Duration
		latencies []time.Duration
	)

	for duration := range results {
		total += duration
		count++
		if duration < min {
			min = duration
		}
		if duration > max {
			max = duration
		}
		latencies = append(latencies, duration)
	}

	// Calculate percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]

	// Print results
	fmt.Printf("Test completed in %v\n", time.Since(start))
	fmt.Printf("Total reads: %d\n", count)
	fmt.Printf("Average latency: %v\n", total/time.Duration(count))
	fmt.Printf("Min latency: %v\n", min)
	fmt.Printf("Max latency: %v\n", max)
	fmt.Printf("P50 latency: %v\n", p50)
	fmt.Printf("P95 latency: %v\n", p95)
	fmt.Printf("P99 latency: %v\n", p99)
}
