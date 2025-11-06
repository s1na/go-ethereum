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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create transaction that calls the selfdestruct contract
			signer := types.HomesteadSigner{}
			tx, err := types.SignTx(types.NewTx(&types.LegacyTx{
				Nonce:    0,
				To:       &contract,
				Value:    big.NewInt(0),
				Gas:      100000,
				GasPrice: big.NewInt(params.InitialBaseFee * 2), // High enough for London fork
				Data:     nil,
			}), signer, key)
			if err != nil {
				t.Fatalf("failed to sign transaction: %v", err)
			}

			// Choose engine based on test requirements
			var engine consensus.Engine
			if tt.useBeacon {
				engine = beacon.New(ethash.NewFaker())
			} else {
				engine = ethash.NewFaker()
			}

			// Generate chain with genesis and block containing the transaction
			_, blocks, _ := core.GenerateChainWithGenesis(tt.genesis, engine, 1, func(i int, b *core.BlockGen) {
				b.AddTx(tx)
			})

			// Create blockchain (this will commit genesis)
			db := rawdb.NewMemoryDatabase()
			blockchain, err := core.NewBlockChain(db, tt.genesis, engine, nil)
			if err != nil {
				t.Fatalf("failed to create blockchain: %v", err)
			}
			defer blockchain.Stop()

			// Import the block
			if _, err := blockchain.InsertChain(blocks); err != nil {
				t.Fatalf("failed to insert chain: %v", err)
			}

			// Create tracer
			tracer := newSelfdestructStateTracer()

			// Get the genesis block from the blockchain to get initial state
			genesis := blockchain.GetBlockByNumber(0)
			if genesis == nil {
				t.Fatalf("failed to get genesis block")
			}

			// Get the block and execute transaction with tracer
			block := blocks[0]
			statedb, err := blockchain.StateAt(genesis.Root())
			if err != nil {
				t.Fatalf("failed to get state: %v", err)
			}

			// Wrap state with hooks
			logState := state.NewHookedState(statedb, tracer.Hooks())

			// Prepare message
			msg, err := core.TransactionToMessage(tx, signer, nil)
			if err != nil {
				t.Fatalf("failed to prepare transaction for tracing: %v", err)
			}

			// Create block context
			context := core.NewEVMBlockContext(block.Header(), blockchain, nil)

			// Execute transaction
			evm := vm.NewEVM(context, logState, tt.genesis.Config, vm.Config{Tracer: tracer.Hooks()})
			tracer.OnTxStart(evm.GetVMContext(), tx, msg.From)
			vmRet, err := core.ApplyMessage(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
			if err != nil {
				t.Fatalf("failed to execute transaction: %v", err)
			}

			// Finalize state - this triggers the selfdestruct hooks
			logState.Finalise(true)

			tracer.OnTxEnd(&types.Receipt{GasUsed: vmRet.UsedGas}, nil)

			// Get trace results
			results := tracer.Accounts()

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

			// Special verification for caller: should have received the selfdestruct balance minus gas costs
			if callerState, ok := results[caller]; ok {
				// Caller should have nonce = 1 (sent 1 tx)
				if callerState.Nonce != 1 {
					t.Errorf("caller nonce mismatch: have %d, want 1", callerState.Nonce)
				}
				// Caller should exist
				if !callerState.Exists {
					t.Errorf("caller should exist")
				}
				// Caller balance should be less than initial (1 ether) + selfdestruct amount (100) due to gas
				expectedMax := new(big.Int).Add(big.NewInt(params.Ether), big.NewInt(100))
				if callerState.Balance.Cmp(expectedMax) >= 0 {
					t.Errorf("caller balance too high (no gas paid?): have %s, expected less than %s",
						callerState.Balance, expectedMax)
				}
				// Caller balance should be at least initial + selfdestruct - reasonable gas limit
				expectedMin := new(big.Int).Add(big.NewInt(params.Ether), big.NewInt(100))
				expectedMin.Sub(expectedMin, new(big.Int).Mul(big.NewInt(100000), big.NewInt(params.InitialBaseFee*2)))
				if callerState.Balance.Cmp(expectedMin) < 0 {
					t.Errorf("caller balance too low: have %s, expected at least %s",
						callerState.Balance, expectedMin)
				}
			}
		})
	}
}
