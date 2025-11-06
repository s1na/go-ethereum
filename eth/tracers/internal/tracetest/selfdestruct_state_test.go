// Copyright 2024 The go-ethereum Authors
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
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// accountState represents the expected final state of an account
type accountState struct {
	Balance *big.Int
	Nonce   uint64
	Code    []byte
	Exists  bool
}

// selfdestructStateTracer tracks state changes during selfdestruct operations
type selfdestructStateTracer struct {
	env      *tracing.VMContext
	accounts map[common.Address]*accountState
}

func newSelfdestructStateTracer() *selfdestructStateTracer {
	return &selfdestructStateTracer{
		accounts: make(map[common.Address]*accountState),
	}
}

func (t *selfdestructStateTracer) OnTxStart(env *tracing.VMContext, tx *types.Transaction, from common.Address) {
	t.env = env
}

func (t *selfdestructStateTracer) OnTxEnd(receipt *types.Receipt, err error) {
	// Nothing to do
}

func (t *selfdestructStateTracer) getOrCreateAccount(addr common.Address) *accountState {
	if acc, ok := t.accounts[addr]; ok {
		return acc
	}

	// Initialize with current state from statedb
	acc := &accountState{
		Balance: t.env.StateDB.GetBalance(addr).ToBig(),
		Nonce:   t.env.StateDB.GetNonce(addr),
		Code:    t.env.StateDB.GetCode(addr),
		Exists:  t.env.StateDB.Exist(addr),
	}
	t.accounts[addr] = acc
	return acc
}

func (t *selfdestructStateTracer) OnBalanceChange(addr common.Address, prev, new *big.Int, reason tracing.BalanceChangeReason) {
	acc := t.getOrCreateAccount(addr)
	acc.Balance = new
}

func (t *selfdestructStateTracer) OnNonceChangeV2(addr common.Address, prev, new uint64, reason tracing.NonceChangeReason) {
	acc := t.getOrCreateAccount(addr)
	acc.Nonce = new

	// If this is a selfdestruct nonce change, mark account as not existing
	if reason == tracing.NonceChangeSelfdestruct {
		acc.Exists = false
	}
}

func (t *selfdestructStateTracer) OnCodeChangeV2(addr common.Address, prevCodeHash common.Hash, prevCode []byte, codeHash common.Hash, code []byte, reason tracing.CodeChangeReason) {
	acc := t.getOrCreateAccount(addr)
	acc.Code = code

	// If this is a selfdestruct code change, mark account as not existing
	if reason == tracing.CodeChangeSelfDestruct {
		acc.Exists = false
	}
}

func (t *selfdestructStateTracer) Hooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnTxStart:       t.OnTxStart,
		OnTxEnd:         t.OnTxEnd,
		OnBalanceChange: t.OnBalanceChange,
		OnNonceChangeV2: t.OnNonceChangeV2,
		OnCodeChangeV2:  t.OnCodeChangeV2,
	}
}

func (t *selfdestructStateTracer) Accounts() map[common.Address]*accountState {
	return t.accounts
}

// setupTestBlockchain creates a blockchain with the given genesis and transaction,
// returns the blockchain, the first block, and a statedb at genesis for testing
func setupTestBlockchain(t *testing.T, genesis *core.Genesis, tx *types.Transaction, useBeacon bool) (*core.BlockChain, *types.Block, *state.StateDB) {
	// Choose engine based on test requirements
	var engine consensus.Engine
	if useBeacon {
		engine = beacon.New(ethash.NewFaker())
	} else {
		engine = ethash.NewFaker()
	}

	// Generate chain with genesis and block containing the transaction
	_, blocks, _ := core.GenerateChainWithGenesis(genesis, engine, 1, func(i int, b *core.BlockGen) {
		b.AddTx(tx)
	})

	// Create blockchain
	db := rawdb.NewMemoryDatabase()
	blockchain, err := core.NewBlockChain(db, genesis, engine, nil)
	if err != nil {
		t.Fatalf("failed to create blockchain: %v", err)
	}

	// Import the block
	if _, err := blockchain.InsertChain(blocks); err != nil {
		t.Fatalf("failed to insert chain: %v", err)
	}

	// Get the genesis block
	genesisBlock := blockchain.GetBlockByNumber(0)
	if genesisBlock == nil {
		t.Fatalf("failed to get genesis block")
	}

	// Get genesis state
	statedb, err := blockchain.StateAt(genesisBlock.Root())
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}

	return blockchain, blocks[0], statedb
}

func TestSelfdestructStateTracer(t *testing.T) {
	t.Parallel()

	var (
		key, _   = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		caller   = crypto.PubkeyToAddress(key.PublicKey)
		contract = common.HexToAddress("0x00000000000000000000000000000000000000bb")

		// Contract code: CALLER SELFDESTRUCT (sends balance to caller)
		selfdestructCode = []byte{
			byte(vm.CALLER),       // Push caller address
			byte(vm.SELFDESTRUCT), // Selfdestruct
		}

		// Factory contract that creates a contract and calls it to selfdestruct
		factory = common.HexToAddress("0x00000000000000000000000000000000000000ff")

		// Factory code (compiled from Yul):
		/*
			object "Factory" {
				code {
					datacopy(0, dataoffset("Runtime"), datasize("Runtime"))
					return(0, datasize("Runtime"))
				}
				object "Runtime" {
					code {
						// Store init code for child (0x6133ff6000526002601ef3 = init code that returns 0x33ff)
						mstore(0, 0x6133ff6000526002601ef3000000000000000000000000000000000000000000)

						// CREATE child contract with 11 bytes of init code starting at byte 0
						let child := create(0, 0, 11)

						// CALL child contract (triggers selfdestruct)
						pop(call(gas(), child, 0, 0, 0, 0, 0))

						stop()
					}
				}
			}
		*/
		// Compiled with: solc --strict-assembly --evm-version paris factory.yul --bin
	// (Using paris to avoid PUSH0 opcode which is not available pre-Shanghai)
		// Runtime bytecode (part after 0xfe in output):
		factoryCode = common.Hex2Bytes("6133ff6000526a6133ff6000526002601ef360a81b600052600080808080600b8180f05af100")

		// The address where the factory will create the contract (factory starts with nonce 0 in genesis)
		createdContractAddr = crypto.CreateAddress(factory, 0)
	)

	tests := []struct {
		name            string
		genesis         *core.Genesis
		useBeacon       bool // Use beacon engine instead of ethash
		expectedResults map[common.Address]accountState
	}{
		{
			name: "pre-EIP-6780: existing contract selfdestructs",
			genesis: &core.Genesis{
				Config: params.AllEthashProtocolChanges,
				Alloc: types.GenesisAlloc{
					caller: {Balance: big.NewInt(params.Ether)},
					contract: {
						Balance: big.NewInt(100),
						Code:    selfdestructCode,
					},
				},
			},
			useBeacon: false, // Use ethash for pre-Shanghai
			expectedResults: map[common.Address]accountState{
				contract: {
					Balance: big.NewInt(0),
					Nonce:   0,
					Code:    []byte{},
					Exists:  false,
				},
				// Note: caller balance will be less than Ether+100 due to gas costs
				// We check this separately in the test
			},
		},
		{
			name: "post-EIP-6780: existing contract selfdestructs (should NOT destroy)",
			genesis: &core.Genesis{
				Config: params.AllDevChainProtocolChanges,
				Alloc: types.GenesisAlloc{
					caller: {Balance: big.NewInt(params.Ether)},
					contract: {
						Balance: big.NewInt(100),
						Code:    selfdestructCode,
					},
				},
			},
			useBeacon: true, // Use beacon engine for Shanghai+
			expectedResults: map[common.Address]accountState{
				contract: {
					Balance: big.NewInt(0),    // Balance transferred
					Nonce:   0,                // Nonce unchanged
					Code:    selfdestructCode, // Code still exists!
					Exists:  true,             // Contract still exists!
				},
				// Note: caller balance will be less than Ether+100 due to gas costs
				// We check this separately in the test
			},
		},
		{
			name: "pre-EIP-6780: contract created and selfdestructed in same tx",
			genesis: &core.Genesis{
				Config: params.AllEthashProtocolChanges,
				Alloc: types.GenesisAlloc{
					caller:  {Balance: big.NewInt(params.Ether)},
					factory: {Code: factoryCode},
				},
			},
			useBeacon: false,
			expectedResults: map[common.Address]accountState{
				createdContractAddr: {
					Balance: big.NewInt(0),
					Nonce:   0,
					Code:    []byte{},
					Exists:  false, // Contract destroyed
				},
			},
		},
		{
			name: "post-EIP-6780: contract created and selfdestructed in same tx (SHOULD destroy)",
			genesis: &core.Genesis{
				Config: params.AllDevChainProtocolChanges,
				Alloc: types.GenesisAlloc{
					caller:  {Balance: big.NewInt(params.Ether)},
					factory: {Code: factoryCode},
				},
			},
			useBeacon: true,
			expectedResults: map[common.Address]accountState{
				createdContractAddr: {
					Balance: big.NewInt(0),
					Nonce:   0,
					Code:    []byte{},
					Exists:  false, // Contract destroyed (EIP-6780 exception: created in same tx)
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			signer := types.HomesteadSigner{}
			var tx *types.Transaction
			var err error

			// Determine if this is a factory test or direct contract call test
			_, hasFactory := tt.genesis.Alloc[factory]
			isCreateAndDestroy := hasFactory

			if isCreateAndDestroy {
				// Call factory to create and destroy contract in same tx
				tx, err = types.SignTx(types.NewTx(&types.LegacyTx{
					Nonce:    0,
					To:       &factory,
					Value:    big.NewInt(0),
					Gas:      200000,
					GasPrice: big.NewInt(params.InitialBaseFee * 2),
					Data:     nil,
				}), signer, key)
			} else {
				// Call existing contract
				tx, err = types.SignTx(types.NewTx(&types.LegacyTx{
					Nonce:    0,
					To:       &contract,
					Value:    big.NewInt(0),
					Gas:      100000,
					GasPrice: big.NewInt(params.InitialBaseFee * 2),
					Data:     nil,
				}), signer, key)
			}
			if err != nil {
				t.Fatalf("failed to sign transaction: %v", err)
			}

			// Setup blockchain with transaction
			blockchain, block, statedb := setupTestBlockchain(t, tt.genesis, tx, tt.useBeacon)
			defer blockchain.Stop()

			// Create tracer
			tracer := newSelfdestructStateTracer()

			// Wrap state with hooks
			hookedState := state.NewHookedState(statedb, tracer.Hooks())

			// Prepare message
			msg, err := core.TransactionToMessage(tx, signer, nil)
			if err != nil {
				t.Fatalf("failed to prepare transaction for tracing: %v", err)
			}

			// Create block context
			context := core.NewEVMBlockContext(block.Header(), blockchain, nil)

			// Execute transaction with EVM (handles OnTxStart, ApplyMessage, Finalise, OnTxEnd)
			evm := vm.NewEVM(context, hookedState, tt.genesis.Config, vm.Config{Tracer: tracer.Hooks()})
			usedGas := uint64(0)
			_, err = core.ApplyTransactionWithEVM(msg, new(core.GasPool).AddGas(tx.Gas()), statedb, block.Number(), block.Hash(), block.Time(), tx, &usedGas, evm)
			if err != nil {
				t.Fatalf("failed to execute transaction: %v", err)
			}
			results := tracer.Accounts()

			// Debug: print all addresses in results
			if isCreateAndDestroy {
				t.Logf("Addresses in tracer results:")
				for addr := range results {
					t.Logf("  - %s", addr.Hex())
				}
			}

			// Verify results
			for addr, expected := range tt.expectedResults {
				actual, ok := results[addr]
				if !ok {
					t.Errorf("address %s missing from results", addr.Hex())
					continue
				}

				if actual.Balance.Cmp(expected.Balance) != 0 {
					t.Errorf("address %s: balance mismatch: have %s, want %s",
						addr.Hex(), actual.Balance, expected.Balance)
				}
				if actual.Nonce != expected.Nonce {
					t.Errorf("address %s: nonce mismatch: have %d, want %d",
						addr.Hex(), actual.Nonce, expected.Nonce)
				}
				if len(actual.Code) != len(expected.Code) {
					t.Errorf("address %s: code length mismatch: have %d, want %d",
						addr.Hex(), len(actual.Code), len(expected.Code))
				}
				if actual.Exists != expected.Exists {
					t.Errorf("address %s: exists mismatch: have %v, want %v",
						addr.Hex(), actual.Exists, expected.Exists)
				}
			}

			// Verify caller balance
			gasCost := new(big.Int).Mul(new(big.Int).SetUint64(usedGas), tx.GasPrice())
			var expectedCallerBalance *big.Int
			if isCreateAndDestroy {
				// Create-and-destroy: initial - gas cost (contract selfdestructed to caller with 0 balance)
				expectedCallerBalance = new(big.Int).Sub(big.NewInt(params.Ether), gasCost)
			} else {
				// Existing contract: initial + transfer - gas cost
				expectedCallerBalance = new(big.Int).Add(big.NewInt(params.Ether), big.NewInt(100))
				expectedCallerBalance.Sub(expectedCallerBalance, gasCost)
			}

			if callerState, ok := results[caller]; ok {
				if callerState.Balance.Cmp(expectedCallerBalance) != 0 {
					t.Errorf("caller balance mismatch: have %s, want %s (gas used: %d)",
						callerState.Balance, expectedCallerBalance, usedGas)
				}
				if callerState.Nonce != 1 {
					t.Errorf("caller nonce mismatch: have %d, want 1", callerState.Nonce)
				}
				if !callerState.Exists {
					t.Errorf("caller should exist")
				}
			}
		})
	}
}
