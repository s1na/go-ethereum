// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package tracetest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tracers/directory"
	"github.com/ethereum/go-ethereum/eth/tracers/live"
	"github.com/ethereum/go-ethereum/params"

	// Force-load live packages, to trigger registration
	_ "github.com/ethereum/go-ethereum/eth/tracers/live"
)

func emptyBlockGenerationFunc(b *core.BlockGen) {}

func TestSupplyGenesisAlloc(t *testing.T) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		funds   = new(big.Int).Mul(common.Big1, big.NewInt(params.Ether))

		config = *params.AllEthashProtocolChanges

		gspec = &core.Genesis{
			Config: &config,
			Alloc: core.GenesisAlloc{
				addr1: {Balance: funds},
				addr2: {Balance: funds},
			},
		}
	)

	expected := live.SupplyInfo{
		Delta:       new(big.Int).Mul(common.Big2, big.NewInt(params.Ether)),
		Reward:      common.Big0,
		Withdrawals: common.Big0,
		Burn:        common.Big0,
		Number:      0,
		Hash:        common.HexToHash("0xbcc9466e9fc6a8b56f4b29ca353a421ff8b51a0c1a58ca4743b427605b08f2ca"), ParentHash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
	}

	out, _, err := testSupplyTracer(gspec, emptyBlockGenerationFunc)
	if err != nil {
		t.Fatalf("failed to test supply tracer: %v", err)
	}

	actual := out[expected.Number]

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("incorrect supply info: expected %+v, got %+v", expected, actual)
	}
}

func TestSupplyRewards(t *testing.T) {
	var (
		config = *params.AllEthashProtocolChanges

		gspec = &core.Genesis{
			Config: &config,
		}
	)

	expected := live.SupplyInfo{
		Delta:       new(big.Int).Mul(common.Big2, big.NewInt(params.Ether)),
		Reward:      new(big.Int).Mul(common.Big2, big.NewInt(params.Ether)),
		Withdrawals: common.Big0,
		Burn:        common.Big0,
		Number:      1,
		Hash:        common.HexToHash("0xcbb08370505be503dafedc4e96d139ea27aba3cbc580148568b8a307b3f51052"), ParentHash: common.HexToHash("0xadeda0a83e337b6c073e3f0e9a17531a04009b397a9588c093b628f21b8bc5a3"),
	}

	out, _, err := testSupplyTracer(gspec, emptyBlockGenerationFunc)
	if err != nil {
		t.Fatalf("failed to test supply tracer: %v", err)
	}

	actual := out[expected.Number]

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("incorrect supply info: expected %+v, got %+v", expected, actual)
	}
}

func TestSupplyEip1559Burn(t *testing.T) {
	var (
		config = *params.AllEthashProtocolChanges

		aa = common.HexToAddress("0x000000000000000000000000000000000000aaaa")
		// A sender who makes transactions, has some funds
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		funds   = new(big.Int).Mul(common.Big1, big.NewInt(params.Ether))

		gspec = &core.Genesis{
			Config:  &config,
			BaseFee: big.NewInt(params.InitialBaseFee),
			Alloc: core.GenesisAlloc{
				addr1: {Balance: funds},
				// The address 0xAAAA sloads 0x00 and 0x01
				aa: {
					Code: []byte{
						byte(vm.PC),
						byte(vm.PC),
						byte(vm.SLOAD),
						byte(vm.SLOAD),
					},
					Nonce:   0,
					Balance: big.NewInt(0),
				},
			},
		}
	)

	signer := types.LatestSigner(gspec.Config)

	eip1559BlockGenerationFunc := func(b *core.BlockGen) {
		// One transaction to 0xAAAA
		accesses := types.AccessList{types.AccessTuple{
			Address:     aa,
			StorageKeys: []common.Hash{{0}},
		}}

		txdata := &types.DynamicFeeTx{
			ChainID:    gspec.Config.ChainID,
			Nonce:      0,
			To:         &aa,
			Gas:        30000,
			GasFeeCap:  new(big.Int).Mul(big.NewInt(5), big.NewInt(params.GWei)),
			GasTipCap:  big.NewInt(2),
			AccessList: accesses,
			Data:       []byte{},
		}
		tx := types.NewTx(txdata)
		tx, _ = types.SignTx(tx, signer, key1)

		b.AddTx(tx)
	}

	expected := live.SupplyInfo{
		Delta:       big.NewInt(1999975934000000000),
		Reward:      new(big.Int).Mul(common.Big2, big.NewInt(params.Ether)),
		Withdrawals: common.Big0,
		Burn:        big.NewInt(24066000000000),
		Number:      1,
		Hash:        common.HexToHash("0x9910ef69af46d01093bd74da6045c0a2d37adf158001d1df5abb4106d85f1aeb"), ParentHash: common.HexToHash("0x7469edd360a63bcf47b7ab6e1a28e5ace54985138d99f100191fef012d73cf32"),
	}

	out, _, err := testSupplyTracer(gspec, eip1559BlockGenerationFunc)
	if err != nil {
		t.Fatalf("failed to test supply tracer: %v", err)
	}

	actual := out[expected.Number]

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("incorrect supply info: expected %+v, got %+v", expected, actual)
	}
}

func TestSupplyWithdrawals(t *testing.T) {
	var (
		merger = consensus.NewMerger(rawdb.NewMemoryDatabase())
		config = *params.AllEthashProtocolChanges

		gspec = &core.Genesis{
			Config: &config,
		}
	)

	shanghaiTime := uint64(0)
	gspec.Config.ShanghaiTime = &shanghaiTime

	// Activate merge since genesis
	merger.ReachTTD()
	merger.FinalizePoS()

	// Set the terminal total difficulty in the config
	gspec.Config.TerminalTotalDifficulty = big.NewInt(0)

	withdrawalsBlockGenerationFunc := func(b *core.BlockGen) {
		b.SetPoS()

		b.AddWithdrawal(&types.Withdrawal{
			Validator: 42,
			Address:   common.Address{0xee},
			Amount:    1337,
		})
	}

	expected := live.SupplyInfo{
		Delta:       big.NewInt(1337000000000),
		Reward:      common.Big0,
		Withdrawals: big.NewInt(1337000000000),
		Burn:        common.Big0,
		Number:      1,
		Hash:        common.HexToHash("0xb85343cee6331f7dcdc1b1c8e698e0e63d47d84d8f81c9058fda830ac9368ec0"), ParentHash: common.HexToHash("0xc8265888895eb7715237be1c8730a1370f8fd8b33b6e4d5400413ebf33e4d3be"),
	}

	out, _, err := testSupplyTracer(gspec, withdrawalsBlockGenerationFunc)
	if err != nil {
		t.Fatalf("failed to test supply tracer: %v", err)
	}

	actual := out[expected.Number]

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("incorrect supply info: expected %+v, got %+v", expected, actual)
	}
}

func TestSupplySelfdestruct(t *testing.T) {
	var (
		merger = consensus.NewMerger(rawdb.NewMemoryDatabase())
		config = *params.AllEthashProtocolChanges

		aa      = common.HexToAddress("0x1111111111111111111111111111111111111111")
		bb      = common.HexToAddress("0x2222222222222222222222222222222222222222")
		dad     = common.HexToAddress("0x0000000000000000000000000000000000000dad")
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		funds   = new(big.Int).Mul(common.Big1, big.NewInt(params.Ether))

		gspec = &core.Genesis{
			Config:  &config,
			BaseFee: big.NewInt(params.InitialBaseFee),
			Alloc: core.GenesisAlloc{
				addr1: {Balance: funds},
				aa: {
					Code: common.FromHex("0x61face60f01b6000527322222222222222222222222222222222222222226000806002600080855af160008103603457600080fd5b60008060008034865af1905060008103604c57600080fd5b5050"),
					// Nonce:   0,
					Balance: big.NewInt(0),
				},
				bb: {
					Code:    common.FromHex("0x6000357fface000000000000000000000000000000000000000000000000000000000000808203602f57610dad80ff5b5050"),
					Nonce:   0,
					Balance: funds,
				},
			},
		}
	)

	// Activate merge since genesis
	merger.ReachTTD()
	merger.FinalizePoS()

	// Set the terminal total difficulty in the config
	gspec.Config.TerminalTotalDifficulty = big.NewInt(0)

	signer := types.LatestSigner(gspec.Config)

	testBlockGenerationFunc := func(b *core.BlockGen) {
		b.SetPoS()

		txdata := &types.LegacyTx{
			Nonce:    0,
			To:       &aa,
			Value:    new(big.Int).Mul(big.NewInt(5), big.NewInt(params.GWei)),
			Gas:      150000,
			GasPrice: new(big.Int).Mul(big.NewInt(5), big.NewInt(params.GWei)),
			Data:     []byte{},
		}

		tx := types.NewTx(txdata)
		tx, _ = types.SignTx(tx, signer, key1)

		b.AddTx(tx)
	}

	// 1. Test pre Cancun
	preCancunOutput, preCancunChain, err := testSupplyTracer(gspec, testBlockGenerationFunc)
	if err != nil {
		t.Fatalf("failed to test supply tracer: %v", err)
	}

	// Check balance at state:
	// 1. 0x0000...000dad has 1 ether
	// 3. A has 0 ether
	// 3. B has 0 ether
	statedb, _ := preCancunChain.State()
	if got, exp := statedb.GetBalance(dad), funds; got.Cmp(exp) != 0 {
		t.Fatalf("Pre-cancun address \"%v\" balance, got %v exp %v\n", dad, got, exp)
	}
	if got, exp := statedb.GetBalance(aa), big.NewInt(0); got.Cmp(exp) != 0 {
		t.Fatalf("Pre-cancun address \"%v\" balance, got %v exp %v\n", aa, got, exp)
	}
	if got, exp := statedb.GetBalance(bb), big.NewInt(0); got.Cmp(exp) != 0 {
		t.Fatalf("Pre-cancun address \"%v\" balance, got %v exp %v\n", bb, got, exp)
	}

	// Check live trace output
	expected := live.SupplyInfo{
		Delta:       big.NewInt(-55294500000000),
		Reward:      common.Big0,
		Withdrawals: common.Big0,
		Burn:        big.NewInt(55294500000000),
		Number:      1,
		Hash:        common.HexToHash("0x624bfe06805a0df0bc180c68bc9c85460e754e26b76de9fa52d2e56a8d416e06"),
		ParentHash:  common.HexToHash("0xdd9fbe877f0b43987d2f0cda0df176b7939be14f33eb5137f16e6eddf4562706"),
	}

	actual := preCancunOutput[expected.Number]

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("incorrect supply info: expected %+v, got %+v", expected, actual)
	}

	// 2. Test post Cancun
	cancunTime := uint64(0)
	gspec.Config.ShanghaiTime = &cancunTime
	gspec.Config.CancunTime = &cancunTime

	postCancunOutput, postCancunChain, err := testSupplyTracer(gspec, testBlockGenerationFunc)
	if err != nil {
		t.Fatalf("failed to test supply tracer: %v", err)
	}

	// Check balance at state:
	// 1. 0x0000...000dad has 1 ether
	// 3. A has 0 ether
	// 3. B has 5 gwei
	statedb, _ = postCancunChain.State()
	if got, exp := statedb.GetBalance(dad), funds; got.Cmp(exp) != 0 {
		t.Fatalf("Post-shanghai address \"%v\" balance, got %v exp %v\n", dad, got, exp)
	}
	if got, exp := statedb.GetBalance(aa), big.NewInt(0); got.Cmp(exp) != 0 {
		t.Fatalf("Post-shanghai address \"%v\" balance, got %v exp %v\n", aa, got, exp)
	}
	if got, exp := statedb.GetBalance(bb), new(big.Int).Mul(big.NewInt(5), big.NewInt(params.GWei)); got.Cmp(exp) != 0 {
		t.Fatalf("Post-shanghai address \"%v\" balance, got %v exp %v\n", bb, got, exp)
	}

	// Check live trace output
	expected = live.SupplyInfo{
		Delta:       big.NewInt(-55289500000000),
		Reward:      common.Big0,
		Withdrawals: common.Big0,
		Burn:        big.NewInt(55289500000000),
		Number:      1,
		Hash:        common.HexToHash("0xc80e2b68ae44dd898b269d965c88f9e2a82d08d98fd8c8b7765a3eeadf1b2464"),
		ParentHash:  common.HexToHash("0x16d2bb0b366d3963bf2d8d75cb4b3bc0f233047c948fa746cbd38ac82bf9cfe9"),
	}

	actual = postCancunOutput[expected.Number]

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("incorrect supply info: expected %+v, got %+v", expected, actual)
	}
}

func testSupplyTracer(genesis *core.Genesis, gen func(*core.BlockGen)) ([]live.SupplyInfo, *core.BlockChain, error) {
	var (
		engine = beacon.New(ethash.NewFaker())
	)

	// Load supply tracer
	tracer, err := directory.LiveDirectory.New("supply")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create call tracer: %v", err)
	}

	chain, err := core.NewBlockChain(rawdb.NewMemoryDatabase(), core.DefaultCacheConfigWithScheme(rawdb.PathScheme), genesis, nil, engine, vm.Config{Tracer: tracer}, nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create tester chain: %v", err)
	}
	defer chain.Stop()

	_, blocks, _ := core.GenerateChainWithGenesis(genesis, engine, 1, func(i int, b *core.BlockGen) {
		b.SetCoinbase(common.Address{1})
		gen(b)
	})

	if n, err := chain.InsertChain(blocks); err != nil {
		return nil, chain, fmt.Errorf("block %d: failed to insert into chain: %v", n, err)
	}

	// Check and compare the results
	// TODO: replace file to pass results
	file, err := os.OpenFile("supply.txt", os.O_RDONLY, 0666)
	if err != nil {
		return nil, chain, fmt.Errorf("failed to open output file: %v", err)
	}
	defer file.Close()

	var output []live.SupplyInfo
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		blockBytes := scanner.Bytes()

		var info live.SupplyInfo
		if err := json.Unmarshal(blockBytes, &info); err != nil {
			return nil, chain, fmt.Errorf("failed to unmarshal result: %v", err)
		}

		output = append(output, info)
	}

	return output, chain, nil
}
