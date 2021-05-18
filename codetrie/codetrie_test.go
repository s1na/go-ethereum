package codetrie

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
)

type TChunk struct {
	fio  uint8
	code string
}

type ChunkifyTest struct {
	Input     string
	Chunks    []TChunk
	ChunkSize uint
}

type MerkleizeTest struct {
	Input    string
	Chunks   []TChunk
	CodeRoot string
}

func TestMerkleize32(t *testing.T) {
	testCases := []MerkleizeTest{
		{
			Input: "6000",
			Chunks: []TChunk{
				{
					fio:  0,
					code: "6000",
				},
			},
			CodeRoot: "fe1d3dcb57f6b06c53aaf7c6d33ae49f132472983798526b61e5a999fb37032d",
		},
		{
			Input: strings.Repeat("6000", 15) + "00", // Len: 31
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 15) + "00",
				},
			},
			CodeRoot: "79556343025dfe010078374be02ca81d2e2e37070bff7cf90870c779a4de579a",
		},
		{
			Input: strings.Repeat("6000", 16), // Len: 32
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 16),
				},
			},
			CodeRoot: "272a60435c819342d5e6003ea022d5777237f4a061e6cfa7887defc134939ca6",
		},
		{
			Input: strings.Repeat("6000", 17), // Len: 34
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 16),
				},
				{
					fio:  0,
					code: "6000",
				},
			},
			CodeRoot: "87aa5ecc9c22f1342f3ce7fd534ed2cdad6c3b9cdf03616704c190aaa4d9cc4e",
		},
		{
			Input: strings.Repeat("58", 31) + "605b" + strings.Repeat("58", 30), // Len: 63
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 31) + "60",
				},
				{
					fio:  1,
					code: "5b" + strings.Repeat("58", 30),
				},
			},
			CodeRoot: "a81b84e49034e54c8da52cc0c99423741ecae9bbfdedb159dfeef93be203b3cb",
		},
		{
			Input: strings.Repeat("58", 31) + "7f" + strings.Repeat("5b", 32) + strings.Repeat("58", 30), // Len: 94
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 31) + "7f",
				},
				{
					fio:  32,
					code: strings.Repeat("5b", 32),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 30),
				},
			},
			CodeRoot: "792f6e1cff9922e35e32b8db08807e9f8af007267565d6cadb4edec84c1fc300",
		},
		{
			// Test of https://etherscan.io/tx/0x8217ac6d4c0578a3d954c6800ab59060a3c376c81ee17c3726bd4bcc7167e12e
			Input: "6060604052341561000f57600080fd5b60405160208061031d8339810160405280805160008054600160a060020a03909216600160a060020a031990921691909117905550506102c9806100546000396000f3006060604052600436106100325763ffffffff60e060020a60003504166362c067678114610034578063c0ee0b8a14610070575b005b341561003f57600080fd5b61005c600160a060020a03600435811690602435166044356100d5565b604051901515815260200160405180910390f35b341561007b57600080fd5b61003260048035600160a060020a03169060248035919060649060443590810190830135806020601f8201819004810201604051908101604052818152929190602084018383808284375094965061029895505050505050565b60008054819081908190600160a060020a0316637bd163f33360405160e060020a63ffffffff8416028152600160a060020a039091166004820152602401602060405180830381600087803b151561012c57600080fd5b5af1151561013957600080fd5b505050604051805190501561028e5760009250600160a060020a038716156102455786915081600160a060020a03166370a082313060405160e060020a63ffffffff8416028152600160a060020a039091166004820152602401602060405180830381600087803b15156101ac57600080fd5b5af115156101b957600080fd5b505050604051805190508511156101d3576000935061028e565b81600160a060020a031663a9059cbb878760405160e060020a63ffffffff8516028152600160a060020a0390921660048301526024820152604401602060405180830381600087803b151561022757600080fd5b5af1151561023457600080fd5b50505060405180519050925061028a565b5083600160a060020a03301631811115610262576000935061028e565b600160a060020a03861681156108fc0282604051600060405180830381858888f19650505050505b8293505b5050509392505050565b5050505600a165627a7a7230582046a5a4b3a9b14ddd4256f8e7eb73e2c2bbd4c592872abc481eac2c78cc12de470029000000000000000000000000c3dd239cdd4ecf76bd7e67f50129c7dd8be5dab6",
			Chunks: []TChunk{
				{
					fio: 0,
					code: "6060604052341561000f57600080fd5b60405160208061031d83398101604052",
				},
				{
					fio: 0,
					code: "80805160008054600160a060020a03909216600160a060020a03199092169190",
				},
				{
					fio: 0,
					code: "9117905550506102c9806100546000396000f300606060405260043610610032",
				},
				{
					fio: 0,
					code: "5763ffffffff60e060020a60003504166362c067678114610034578063c0ee0b",
				},
				{
					fio: 1,
					code: "8a14610070575b005b341561003f57600080fd5b61005c600160a060020a0360",
				},
				{
					fio: 1,
					code: "0435811690602435166044356100d5565b604051901515815260200160405180",
				},
				{
					fio: 0,
					code: "910390f35b341561007b57600080fd5b61003260048035600160a060020a0316",
				},
				{
					fio: 6,
					code: "9060248035919060649060443590810190830135806020601f82018190048102",
				},
				{
					fio: 0,
					code: "0160405190810160405281815292919060208401838380828437509496506102",
				},
				{
					fio: 1,
					code: "9895505050505050565b60008054819081908190600160a060020a0316637bd1",
				},
				{
					fio: 27,
					code: "63f33360405160e060020a63ffffffff8416028152600160a060020a03909116",
				},
				{
					fio: 0,
					code: "6004820152602401602060405180830381600087803b151561012c57600080fd",
				},
				{
					fio: 0,
					code: "5b5af1151561013957600080fd5b505050604051805190501561028e57600092",
				},
				{
					fio: 0,
					code: "50600160a060020a038716156102455786915081600160a060020a03166370a0",
				},
				{
					fio: 16,
					code: "82313060405160e060020a63ffffffff8416028152600160a060020a03909116",
				},
				{
					fio: 0,
					code: "6004820152602401602060405180830381600087803b15156101ac57600080fd",
				},
				{
					fio: 0,
					code: "5b5af115156101b957600080fd5b505050604051805190508511156101d35760",
				},
				{
					fio: 1,
					code: "00935061028e565b81600160a060020a031663a9059cbb878760405160e06002",
				},
				{
					fio: 0,
					code: "0a63ffffffff8516028152600160a060020a0390921660048301526024820152",
				},
				{
					fio: 0,
					code: "604401602060405180830381600087803b151561022757600080fd5b5af11515",
				},
				{
					fio: 0,
					code: "61023457600080fd5b50505060405180519050925061028a565b5083600160a0",
				},
				{
					fio: 0,
					code: "60020a03301631811115610262576000935061028e565b600160a060020a0386",
				},
				{
					fio: 0,
					code: "1681156108fc0282604051600060405180830381858888f19650505050505b82",
				},
				{
					fio: 0,
					code: "93505b5050509392505050565b5050505600a165627a7a7230582046a5a4b3a9",
				},
				{
					fio: 11,
					code: "b14ddd4256f8e7eb73e2c2bbd4c592872abc481eac2c78cc12de470029000000",
				},
				{
					fio: 16,
					code: "000000000000000000c3dd239cdd4ecf76bd7e67f50129c7dd8be5dab6",
				},
			},
			CodeRoot: "64de863ab0272175abd6d9014ebcd4e72b794fe22477da118cda27c05345eb11",
		},
	}

	for _, c := range testCases {
		code, err := hex.DecodeString(c.Input)
		if err != nil {
			t.Error(err)
		}
		chunks := Chunkify(code, 32)
		if len(chunks) != len(c.Chunks) {
			t.Errorf("%v: invalid number of chunks: expected %d, got %d\n", t.Name(), len(c.Chunks), len(chunks))
		}
		for i, chunk := range chunks {
			expectedChunk := c.Chunks[i]
			if chunk.fio != expectedChunk.fio {
				t.Errorf("%v: invalid chunk FIO: expected %d, got %d\n", t.Name(), expectedChunk.fio, chunk.fio)
			}
			expectedCode, err := hex.DecodeString(expectedChunk.code)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(chunk.code, expectedCode) {
				t.Errorf("%v: invalid chunk code: expected %s, got %s\n", t.Name(), expectedChunk.code, hex.EncodeToString(chunk.code))
			}
		}

		expectedRoot, err := hex.DecodeString(c.CodeRoot)
		if err != nil {
			t.Error(err)
		}

		root, err := MerkleizeInMemory(code, 32)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(root.Bytes(), expectedRoot) {
			t.Errorf("%v: invalid code root: expected %s, got %s\n", t.Name(), c.CodeRoot, root.Hex())
		}

		// Test StackTrie impl
		stackRoot, err := MerkleizeStack(code, 32)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(stackRoot.Bytes(), expectedRoot) {
			t.Errorf("%v: invalid code root for MerkleizeStack: expected %s, got %s\n", t.Name(), c.CodeRoot, stackRoot.Hex())
		}
	}
}

func TestMerkleize24(t *testing.T) {
	testCases := []MerkleizeTest{
		{
			Input: "6000",
			Chunks: []TChunk{
				{
					fio:  0,
					code: "6000",
				},
			},
			CodeRoot: "fe1d3dcb57f6b06c53aaf7c6d33ae49f132472983798526b61e5a999fb37032d",
		},
		{
			Input: strings.Repeat("6000", 15) + "00", // Len: 31
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 12),
				},
				{
					fio:  0,
					code: strings.Repeat("6000", 3) + "00",
				},
			},
			CodeRoot: "a983e1f939935bf5c32bc34f3c6e388d83d0a5283b3f60faa790fa0908791902",
		},
		{
			Input: strings.Repeat("6000", 16), // Len: 32
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 12),
				},
				{
					fio:  0,
					code: strings.Repeat("6000", 4),
				},
			},
			CodeRoot: "7965257957593eb9061d320e17ef59feec179cfebf93cbb25cfe51898ef6c918",
		},
		{
			Input: strings.Repeat("6000", 17), // Len: 34
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("6000", 12),
				},
				{
					fio:  0,
					code: strings.Repeat("6000", 5),
				},
			},
			CodeRoot: "b4d1d2394738ccdde1ad0dcc38924a7b68767d59a001d83af0e3a32836638800",
		},
		{
			Input: strings.Repeat("58", 31) + "605b" + strings.Repeat("58", 30), // Len: 63
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 24),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 7) + "605b" + strings.Repeat("58", 15),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 15),
				},
			},
			CodeRoot: "a9e011b8aa7f7fd15c0d2918d2d604a6dc489c2e90f5dfbcf3d43fc943fb6950",
		},
		{
			Input: strings.Repeat("58", 31) + "7f" + strings.Repeat("5b", 32) + strings.Repeat("58", 30), // Len: 94
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 24),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 7) + "7f" + strings.Repeat("5b", 16),
				},
				{
					fio:  16,
					code: strings.Repeat("5b", 16) + strings.Repeat("58", 8),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 22),
				},
			},
			CodeRoot: "f41f007c3e0341a908c42cc854bbf2b61dbf97e5bd5e65b4867c6be80c9efc6a",
		},
		{
			// Oversized push contains a PUSH32 byte
			Input: "7f" + strings.Repeat("58", 23) + "7f" + strings.Repeat("58", 8) + "60006000fe",
			Chunks: []TChunk{
				{
					fio: 0,
					code: "7f" + strings.Repeat("58", 23),
				},
				{
					fio: 9,
					code: "7f" + strings.Repeat("58", 8) + "60006000fe",
				},
			},
			CodeRoot: "fa2bf0e8ed8d155b3fd38010b81c6381ab2f9b5daf141ce625c3913ba0ac65fb",
		},
		{
			// Oversized push spanning 3 chunks
			Input: strings.Repeat("58", 23) + "7f" + strings.Repeat("58", 23) + "7f7f" + strings.Repeat("58", 7) + "60006000fe",
			Chunks: []TChunk{
				{
					fio: 0,
					code: strings.Repeat("58", 23) + "7f",
				},
				{
					fio: 24,
					code: strings.Repeat("58", 23) + "7f",
				},
				{
					fio: 8,
					code: "7f" + strings.Repeat("58", 7) + "60006000fe",
				},
			},
			CodeRoot: "3788a496364cc5f3263c195f5ed633cf5e96fa89aa7521c946b85bc3e94f1ec9",
		},
		{
			// Push data is truncated (i.e. Solidity metadata)
			Input: strings.Repeat("58", 23) + "7f" + strings.Repeat("58", 3),
			Chunks: []TChunk{
				{
					fio: 0,
					code: strings.Repeat("58", 23) + "7f",
				},
				{
					fio: 24,
					code: strings.Repeat("58", 3),
				},
			},
			CodeRoot: "b2cf2c424ea0e1b43b0d2d2fb9b2b4983a9aac4a2319af47806e6b9625767a44",
		},
		{
			// Test of https://etherscan.io/tx/0x8217ac6d4c0578a3d954c6800ab59060a3c376c81ee17c3726bd4bcc7167e12e
			Input: "6060604052341561000f57600080fd5b60405160208061031d8339810160405280805160008054600160a060020a03909216600160a060020a031990921691909117905550506102c9806100546000396000f3006060604052600436106100325763ffffffff60e060020a60003504166362c067678114610034578063c0ee0b8a14610070575b005b341561003f57600080fd5b61005c600160a060020a03600435811690602435166044356100d5565b604051901515815260200160405180910390f35b341561007b57600080fd5b61003260048035600160a060020a03169060248035919060649060443590810190830135806020601f8201819004810201604051908101604052818152929190602084018383808284375094965061029895505050505050565b60008054819081908190600160a060020a0316637bd163f33360405160e060020a63ffffffff8416028152600160a060020a039091166004820152602401602060405180830381600087803b151561012c57600080fd5b5af1151561013957600080fd5b505050604051805190501561028e5760009250600160a060020a038716156102455786915081600160a060020a03166370a082313060405160e060020a63ffffffff8416028152600160a060020a039091166004820152602401602060405180830381600087803b15156101ac57600080fd5b5af115156101b957600080fd5b505050604051805190508511156101d3576000935061028e565b81600160a060020a031663a9059cbb878760405160e060020a63ffffffff8516028152600160a060020a0390921660048301526024820152604401602060405180830381600087803b151561022757600080fd5b5af1151561023457600080fd5b50505060405180519050925061028a565b5083600160a060020a03301631811115610262576000935061028e565b600160a060020a03861681156108fc0282604051600060405180830381858888f19650505050505b8293505b5050509392505050565b5050505600a165627a7a7230582046a5a4b3a9b14ddd4256f8e7eb73e2c2bbd4c592872abc481eac2c78cc12de470029000000000000000000000000c3dd239cdd4ecf76bd7e67f50129c7dd8be5dab6",
			Chunks: []TChunk{
				{
					fio: 0,
					code: "6060604052341561000f57600080fd5b6040516020806103",
				},
				{
					fio: 1,
					code: "1d8339810160405280805160008054600160a060020a0390",
				},
				{
					fio: 0,
					code: "9216600160a060020a031990921691909117905550506102",
				},
				{
					fio: 1,
					code: "c9806100546000396000f300606060405260043610610032",
				},
				{
					fio: 0,
					code: "5763ffffffff60e060020a60003504166362c06767811461",
				},
				{
					fio: 2,
					code: "0034578063c0ee0b8a14610070575b005b341561003f5760",
				},
				{
					fio: 1,
					code: "0080fd5b61005c600160a060020a03600435811690602435",
				},
				{
					fio: 0,
					code: "166044356100d5565b604051901515815260200160405180",
				},
				{
					fio: 0,
					code: "910390f35b341561007b57600080fd5b6100326004803560",
				},
				{
					fio: 1,
					code: "0160a060020a031690602480359190606490604435908101",
				},
				{
					fio: 0,
					code: "90830135806020601f820181900481020160405190810160",
				},
				{
					fio: 1,
					code: "405281815292919060208401838380828437509496506102",
				},
				{
					fio: 1,
					code: "9895505050505050565b60008054819081908190600160a0",
				},
				{
					fio: 0,
					code: "60020a0316637bd163f33360405160e060020a63ffffffff",
				},
				{
					fio: 11,
					code: "8416028152600160a060020a039091166004820152602401",
				},
				{
					fio: 0,
					code: "602060405180830381600087803b151561012c57600080fd",
				},
				{
					fio: 0,
					code: "5b5af1151561013957600080fd5b50505060405180519050",
				},
				{
					fio: 0,
					code: "1561028e5760009250600160a060020a0387161561024557",
				},
				{
					fio: 0,
					code: "86915081600160a060020a03166370a082313060405160e0",
				},
				{
					fio: 8,
					code: "60020a63ffffffff8416028152600160a060020a03909116",
				},
				{
					fio: 0,
					code: "6004820152602401602060405180830381600087803b1515",
				},
				{
					fio: 0,
					code: "6101ac57600080fd5b5af115156101b957600080fd5b5050",
				},
				{
					fio: 0,
					code: "50604051805190508511156101d3576000935061028e565b",
				},
				{
					fio: 0,
					code: "81600160a060020a031663a9059cbb878760405160e06002",
				},
				{
					fio: 0,
					code: "0a63ffffffff8516028152600160a060020a039092166004",
				},
				{
					fio: 0,
					code: "830152602482015260440160206040518083038160008780",
				},
				{
					fio: 0,
					code: "3b151561022757600080fd5b5af1151561023457600080fd",
				},
				{
					fio: 0,
					code: "5b50505060405180519050925061028a565b5083600160a0",
				},
				{
					fio: 0,
					code: "60020a03301631811115610262576000935061028e565b60",
				},
				{
					fio: 1,
					code: "0160a060020a03861681156108fc02826040516000604051",
				},
				{
					fio: 0,
					code: "80830381858888f19650505050505b8293505b5050509392",
				},
				{
					fio: 0,
					code: "505050565b5050505600a165627a7a7230582046a5a4b3a9",
				},
				{
					fio: 11,
					code: "b14ddd4256f8e7eb73e2c2bbd4c592872abc481eac2c78cc",
				},
				{
					fio: 24,
					code: "12de470029000000000000000000000000c3dd239cdd4ecf",
				},
				{
					fio: 0,
					code: "76bd7e67f50129c7dd8be5dab6",
				},
			},
			CodeRoot: "aa90663202260f3ee4a782c1b4d5edd94d88b1ea14dcc67bc6548e3a7f848be6",
		},
	}

	for _, c := range testCases {
		code, err := hex.DecodeString(c.Input)
		if err != nil {
			t.Error(err)
		}
		chunks := Chunkify(code, 24)
		if len(chunks) != len(c.Chunks) {
			t.Errorf("%v: invalid number of chunks: expected %d, got %d\n", t.Name(), len(c.Chunks), len(chunks))
		}
		for i, chunk := range chunks {
			expectedChunk := c.Chunks[i]
			if chunk.fio != expectedChunk.fio {
				t.Errorf("%v: invalid chunk FIO: expected %d, got %d\n", t.Name(), expectedChunk.fio, chunk.fio)
			}
			expectedCode, err := hex.DecodeString(expectedChunk.code)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(chunk.code, expectedCode) {
				t.Errorf("%v: invalid chunk code: expected %s, got %s\n", t.Name(), expectedChunk.code, hex.EncodeToString(chunk.code))
			}
		}

		expectedRoot, err := hex.DecodeString(c.CodeRoot)
		if err != nil {
			t.Error(err)
		}

		root, err := MerkleizeInMemory(code, 24)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(root.Bytes(), expectedRoot) {
			t.Errorf("%v: invalid code root: expected %s, got %s\n", t.Name(), c.CodeRoot, root.Hex())
		}

		// Test StackTrie impl
		stackRoot, err := MerkleizeStack(code, 24)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(stackRoot.Bytes(), expectedRoot) {
			t.Errorf("%v: invalid code root for MerkleizeStack: expected %s, got %s\n", t.Name(), c.CodeRoot, stackRoot.Hex())
		}
	}
}

func TestChunkifySize(t *testing.T) {
	testCases := []ChunkifyTest{
		{
			ChunkSize: 48,
			Input:     strings.Repeat("58", 47) + "7f" + strings.Repeat("5b", 32) + strings.Repeat("58", 30),
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 47) + "7f",
				},
				{
					fio:  32,
					code: strings.Repeat("5b", 32) + strings.Repeat("58", 16),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 14),
				},
			},
		},
		{
			ChunkSize: 16,
			Input:     strings.Repeat("58", 15) + "7f" + strings.Repeat("5b", 32) + strings.Repeat("58", 10),
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 15) + "7f",
				},
				{
					fio:  16,
					code: strings.Repeat("5b", 16),
				},
				{
					fio:  16,
					code: strings.Repeat("5b", 16),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 10),
				},
			},
		},
		{
			ChunkSize: 8,
			Input:     strings.Repeat("58", 6) + "6b" + strings.Repeat("5b", 12) + strings.Repeat("58", 8),
			Chunks: []TChunk{
				{
					fio:  0,
					code: strings.Repeat("58", 6) + "6b" + "5b",
				},
				{
					fio:  8,
					code: strings.Repeat("5b", 8),
				},
				{
					fio:  3,
					code: strings.Repeat("5b", 3) + strings.Repeat("58", 5),
				},
				{
					fio:  0,
					code: strings.Repeat("58", 3),
				},
			},
		},
	}

	for _, c := range testCases {
		code, err := hex.DecodeString(c.Input)
		if err != nil {
			t.Error(err)
		}
		chunks := Chunkify(code, c.ChunkSize)
		if len(chunks) != len(c.Chunks) {
			t.Errorf("%v: invalid number of chunks: expected %d, got %d\n", t.Name(), len(c.Chunks), len(chunks))
		}
		for i, chunk := range chunks {
			expectedChunk := c.Chunks[i]
			if chunk.fio != expectedChunk.fio {
				t.Errorf("%v: invalid chunk FIO: expected %d, got %d\n", t.Name(), expectedChunk.fio, chunk.fio)
			}
			expectedCode, err := hex.DecodeString(expectedChunk.code)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(chunk.code, expectedCode) {
				t.Errorf("%v: invalid chunk code: expected %s, got %s\n", t.Name(), expectedChunk.code, hex.EncodeToString(chunk.code))
			}
		}
	}
}

func BenchmarkChunkify(b *testing.B) {
	code := getSampleContract(b)
	b.Logf("CodeLen: %v\n", len(code))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Chunkify(code, 32)
	}
}

func BenchmarkOverhead(b *testing.B) {
	code := getSampleContract(b)
	b.Logf("CodeLen: %v\n", len(code))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MerkleizeStack(code, 32)
	}
}

func BenchmarkNewMemoryDb(b *testing.B) {
	for i := 0; i < b.N; i++ {
		memorydb.New()
	}
}

func BenchmarkNoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}

func BenchmarkKeccak(b *testing.B) {
	data := "7624778dedc75f8b322b9fa1632a610d40b85e106c7d9bf0e743a9ce291b9c6f"
	input, _ := hex.DecodeString(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crypto.Keccak256Hash(input)
	}
}

func getSampleContract(b *testing.B) []byte {
	type ContractStat struct {
		CodeLen  int
		Code     string
		Duration int64
	}
	type Schema struct {
		Stats []ContractStat
	}
	f, err := ioutil.ReadFile("../contracts.json")
	if err != nil {
		b.Errorf("%v: failed reading contracts file. Got error: %v\n", b.Name(), err)
	}
	var data Schema
	if err := json.Unmarshal(f, &data); err != nil {
		b.Errorf("%v: failed unmarshalling json: %v\n", b.Name(), err)
	}

	codeHex := data.Stats[0].Code
	code, err := hex.DecodeString(codeHex)
	if err != nil {
		b.Errorf("%v: failed decoding code hex: %v\n", b.Name(), err)
	}

	return code
}
