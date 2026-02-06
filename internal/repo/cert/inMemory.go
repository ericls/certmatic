package cert

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/ericls/certmatic/pkg/cert"
)

type entry struct {
	data     []byte
	modified int64
}

type InMemoryCertRepo struct {
	mu      sync.RWMutex
	data    map[string]*entry
	locks   map[string]bool
	locksMu sync.Mutex
}

func NewInMemoryCertRepo() *InMemoryCertRepo {
	return &InMemoryCertRepo{
		data:  make(map[string]*entry),
		locks: make(map[string]bool),
	}
}

// Get retrieves a value by key. Returns ErrNotExist if the key doesn't exist.
func (r *InMemoryCertRepo) Get(ctx context.Context, key string) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, exists := r.data[key]
	if !exists {
		return nil, cert.ErrNotExist
	}
	// Return a copy to prevent external mutation
	result := make([]byte, len(e.data))
	copy(result, e.data)
	return result, nil
}

// Put stores a value with the given key.
func (r *InMemoryCertRepo) Put(ctx context.Context, key string, value []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store a copy to prevent external mutation
	data := make([]byte, len(value))
	copy(data, value)
	r.data[key] = &entry{
		data:     data,
		modified: time.Now().Unix(),
	}
	return nil
}

// Delete removes a key. Returns nil if the key doesn't exist.
func (r *InMemoryCertRepo) Delete(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.data, key)
	return nil
}

// Exists checks if a key exists.
func (r *InMemoryCertRepo) Exists(ctx context.Context, key string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.data[key]
	return exists, nil
}

// List returns all keys with the given prefix.
func (r *InMemoryCertRepo) List(ctx context.Context, prefix string, recursive bool) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []string
	seen := make(map[string]bool)

	for key := range r.data {
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		suffix := strings.TrimPrefix(key, prefix)
		suffix = strings.TrimPrefix(suffix, "/")

		if recursive {
			results = append(results, key)
		} else {
			// Non-recursive: return immediate children only
			parts := strings.SplitN(suffix, "/", 2)
			if len(parts) > 0 && parts[0] != "" {
				child := prefix + "/" + parts[0]
				if prefix == "" {
					child = parts[0]
				}
				if !seen[child] {
					seen[child] = true
					results = append(results, child)
				}
			}
		}
	}
	return results, nil
}

// Stat returns information about a key.
func (r *InMemoryCertRepo) Stat(ctx context.Context, key string) (cert.KeyInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, exists := r.data[key]
	if exists {
		return cert.KeyInfo{
			Key:        key,
			Size:       int64(len(e.data)),
			Modified:   e.modified,
			IsTerminal: true,
		}, nil
	}

	// Check if it's a directory (prefix)
	prefix := key + "/"
	for k := range r.data {
		if strings.HasPrefix(k, prefix) {
			return cert.KeyInfo{
				Key:        key,
				Size:       0,
				Modified:   0,
				IsTerminal: false,
			}, nil
		}
	}

	return cert.KeyInfo{}, cert.ErrNotExist
}

// Lock acquires a lock on a key.
func (r *InMemoryCertRepo) Lock(ctx context.Context, key string) error {
	r.locksMu.Lock()
	defer r.locksMu.Unlock()

	if r.locks[key] {
		return cert.ErrLocked
	}
	r.locks[key] = true
	return nil
}

// Unlock releases a lock on a key.
func (r *InMemoryCertRepo) Unlock(ctx context.Context, key string) error {
	r.locksMu.Lock()
	defer r.locksMu.Unlock()

	if !r.locks[key] {
		return cert.ErrLockNotHeld
	}
	delete(r.locks, key)
	return nil
}
