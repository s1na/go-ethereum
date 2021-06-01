package verkle

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var (
	SnapshotLeafPrefix = []byte("l") // Same prefix for both accounts and storage slots
)

type LeafIterator interface {
	snapshot.Iterator

	Leaf() []byte
}

type leafIterator struct {
	it ethdb.Iterator
}

func NewLeafIterator(db ethdb.KeyValueStore) LeafIterator {
	return &leafIterator{
		it: db.NewIterator(SnapshotLeafPrefix, nil),
	}
}

func (it *leafIterator) Next() bool {
	// If the iterator was already exhausted, don't bother
	if it.it == nil {
		return false
	}
	// Try to advance the iterator and release it if we reached the end
	for {
		if !it.it.Next() {
			it.it.Release()
			it.it = nil
			return false
		}
		if len(it.it.Key()) == len(SnapshotLeafPrefix)+common.HashLength {
			break
		}
	}
	return true
}

func (it *leafIterator) Error() error {
	if it.it == nil {
		return nil // Iterator is exhausted and released
	}
	return it.it.Error()
}

func (it *leafIterator) Hash() common.Hash {
	return common.BytesToHash(it.it.Key()) // The prefix will be truncated
}

func (it *leafIterator) Leaf() []byte {
	return it.it.Value()
}

func (it *leafIterator) Release() {
	// The iterator is auto-released on exhaustion, so make sure it's still alive
	if it.it != nil {
		it.it.Release()
		it.it = nil
	}
}

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
