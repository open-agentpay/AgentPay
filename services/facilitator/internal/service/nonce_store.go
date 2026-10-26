package service

import (
	"fmt"
	"strings"
	"sync"
)

// NonceStore tracks used nonces in memory with thread safety.
type NonceStore interface {
	CheckAndRecord(network, asset, payer, nonce string) (bool, error)
	IsUsed(network, asset, payer, nonce string) bool
}

// MemoryNonceStore provides thread-safe in-memory nonce tracking.
type MemoryNonceStore struct {
	mu     sync.RWMutex
	nonces map[string]bool
}

// NewMemoryNonceStore creates an initialized MemoryNonceStore.
func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{
		nonces: make(map[string]bool),
	}
}

func makeNonceKey(network, asset, payer, nonce string) string {
	return fmt.Sprintf("%s:%s:%s:%s",
		strings.ToLower(strings.TrimSpace(network)),
		strings.ToLower(strings.TrimSpace(asset)),
		strings.ToLower(strings.TrimSpace(payer)),
		strings.ToLower(strings.TrimSpace(nonce)),
	)
}

func (s *MemoryNonceStore) CheckAndRecord(network, asset, payer, nonce string) (bool, error) {
	key := makeNonceKey(network, asset, payer, nonce)
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nonces[key] {
		return false, nil // Already used!
	}
	s.nonces[key] = true
	return true, nil
}

func (s *MemoryNonceStore) IsUsed(network, asset, payer, nonce string) bool {
	key := makeNonceKey(network, asset, payer, nonce)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nonces[key]
}
