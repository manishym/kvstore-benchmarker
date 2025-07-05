package runner

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
)

// KeyGenerator generates keys and values for benchmarking
type KeyGenerator struct {
	keys     [][]byte
	mu       sync.RWMutex
	keyIndex int
}

// NewKeyGenerator creates a new key generator with pre-generated keys
func NewKeyGenerator(keySpace int) (*KeyGenerator, error) {
	keys := make([][]byte, keySpace)

	for i := 0; i < keySpace; i++ {
		// Generate 8-16 byte random keys
		keyLen := 8 + (i % 9) // Varies between 8-16 bytes
		key, err := generateRandomBytes(keyLen)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key %d: %w", i, err)
		}
		keys[i] = key
	}

	return &KeyGenerator{
		keys:     keys,
		keyIndex: 0,
	}, nil
}

// GetNextKey returns the next key in round-robin fashion
func (kg *KeyGenerator) GetNextKey() []byte {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	key := kg.keys[kg.keyIndex]
	kg.keyIndex = (kg.keyIndex + 1) % len(kg.keys)
	return key
}

// GetRandomKey returns a random key from the pool
func (kg *KeyGenerator) GetRandomKey() []byte {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	// Use crypto/rand for better randomness
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(kg.keys))))
	if err != nil {
		// Fallback to simple modulo if crypto/rand fails
		n = big.NewInt(int64(kg.keyIndex))
	}

	return kg.keys[n.Int64()]
}

// GenerateValue generates a random value of the specified size
func GenerateValue(size int) ([]byte, error) {
	return generateRandomBytes(size)
}

// generateRandomBytes generates a random byte slice of the specified length
func generateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}

// PreloadKeys preloads the key-value store with data for benchmarking
func PreloadKeys(client interface {
	Put(ctx context.Context, key, value string) error
}, keyGen *KeyGenerator, numKeys int, valueSize int) error {
	// This would be implemented to preload the store with data
	// For now, we'll just return nil as this is a placeholder
	// In a real implementation, you would:
	// 1. Generate keys and values
	// 2. Call Put operations to populate the store
	// 3. Wait for completion
	return nil
}
