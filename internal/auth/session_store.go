package auth

import (
	"sync"
	"time"
)

type sessionEntry struct {
	ID        string
	Username  string
	ExpiresAt time.Time
}

type SessionStore struct {
	mu          sync.RWMutex
	entries     map[string]sessionEntry
	stopCleanup chan struct{}
}

func NewSessionStore() *SessionStore {
	store := &SessionStore{
		entries:     make(map[string]sessionEntry),
		stopCleanup: make(chan struct{}),
	}
	go store.cleanupLoop()
	return store
}

func (store *SessionStore) Create(id string, username string, ttl time.Duration) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.entries[id] = sessionEntry{
		ID:        id,
		Username:  username,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (store *SessionStore) Get(id string) (sessionEntry, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	entry, ok := store.entries[id]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return sessionEntry{}, false
	}
	return entry, true
}

func (store *SessionStore) Delete(id string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.entries, id)
}

func (store *SessionStore) DeleteAll() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.entries = make(map[string]sessionEntry)
}

func (store *SessionStore) DeleteAllExcept(id string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if id == "" {
		store.entries = make(map[string]sessionEntry)
		return
	}
	entry, ok := store.entries[id]
	store.entries = make(map[string]sessionEntry)
	if ok {
		store.entries[id] = entry
	}
}

func (store *SessionStore) Stop() {
	close(store.stopCleanup)
}

func (store *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			store.evictExpired()
		case <-store.stopCleanup:
			return
		}
	}
}

func (store *SessionStore) evictExpired() {
	now := time.Now()
	store.mu.Lock()
	defer store.mu.Unlock()
	for id, entry := range store.entries {
		if now.After(entry.ExpiresAt) {
			delete(store.entries, id)
		}
	}
}
