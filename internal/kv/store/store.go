package store

import (
	"sync"
	"time"
)

// the calling backend server should deal with parsing the value to required type
type storeEntry struct {
	value     []byte
	expiresAt int64 // in unix nanoseconds since epoch, 0 => never expires
}

type KVStore struct {
	mutex sync.RWMutex
	store map[string]storeEntry
}

func New() *KVStore {
	return &KVStore{
		store: make(map[string]storeEntry),
	}
}

func (entry *storeEntry) isExpired() bool {
	if entry.expiresAt == 0 {
		return false
	}

	return entry.expiresAt <= time.Now().UnixNano()
}

// returns true on success, false otherwise
func (kvs *KVStore) Get(key string) ([]byte, bool) {
	kvs.mutex.RLock()
	entry, ok := kvs.store[key]
	kvs.mutex.RUnlock()

	if !ok {
		return nil, false
	}

	if entry.isExpired() {
		kvs.mutex.Lock()
		// recheck for avoiding race conditions
		if current, stillExists := kvs.store[key]; stillExists {
			if current.expiresAt == entry.expiresAt {
				delete(kvs.store, key)
			}
		}
		kvs.mutex.Unlock()

		return nil, false
	}

	// to prevent external mutation of the store, return a copy only
	valueCopy := make([]byte, len(entry.value))
	copy(valueCopy, entry.value)
	return valueCopy, true
}

// expiresAt should be in unix nanoseconds since epoch.
// Note: expiresAt = 0 means never expires
func (kvs *KVStore) Set(key string, value []byte, expiresAt int64) {
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	kvs.mutex.Lock()
	kvs.store[key] = storeEntry{
		value:     valueCopy,
		expiresAt: expiresAt,
	}
	kvs.mutex.Unlock()
}

func (kvs *KVStore) Delete(key string) {
	kvs.mutex.Lock()
	delete(kvs.store, key)
	kvs.mutex.Unlock()
}

func (kvs *KVStore) Clear() {
	kvs.mutex.Lock()
	// replace old map with new map and let GC handle it
	kvs.store = make(map[string]storeEntry)
	kvs.mutex.Unlock()
}
