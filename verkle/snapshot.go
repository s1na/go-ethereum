package verkle

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var (
	SnapshotLeafPrefix = []byte("l") // Same prefix for both accounts and storage slots
)

func WriteAccountSnapshot(db ethdb.KeyValueWriter, hash common.Hash, entry []byte) {
	if err := db.Put(accountSnapshotKey(hash), entry); err != nil {
		log.Crit("Failed to store account snapshot", "err", err)
	}
}

// WriteStorageSnapshot stores the snapshot entry of an storage trie leaf.
func WriteStorageSnapshot(db ethdb.KeyValueWriter, accountHash, storageHash common.Hash, entry []byte) {
	if err := db.Put(storageSnapshotKey(accountHash, storageHash), entry); err != nil {
		log.Crit("Failed to store storage snapshot", "err", err)
	}
}

// accountSnapshotKey = SnapshotAccountPrefix + hash
func accountSnapshotKey(hash common.Hash) []byte {
	return append(SnapshotLeafPrefix, hash.Bytes()...)
}

// storageSnapshotKey = SnapshotStoragePrefix + account hash + storage hash
func storageSnapshotKey(accountHash, storageHash common.Hash) []byte {
	skey := storageKey(accountHash, storageHash)
	return append(SnapshotLeafPrefix, skey...)
}

func storageKey(accountHash, storageHash common.Hash) []byte {
	skeyHi := storageHash.Bytes()[:30]
	skeyLo := storageHash.Bytes()[30:]
	h := sha256.Sum256(append(append(accountHash.Bytes(), byte(1)), skeyHi...))
	return append(append(accountHash.Bytes()[:3], h[:]...), skeyLo...)
}
