package cert

import (
	"context"
	"errors"
)

var (
	ErrNotExist    = errors.New("key does not exist")
	ErrLocked      = errors.New("key is locked")
	ErrLockNotHeld = errors.New("lock not held")
)

// CertRepo defines the interface for certificate and ACME data storage.
type CertRepo interface {
	// Returns ErrNotExist if the key doesn't exist.
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, value []byte) error

	// Returns nil if the key doesn't exist.
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Caller is responsible to not include a trailing slash in prefix.
	List(ctx context.Context, prefix string, recursive bool) ([]string, error)
	Stat(ctx context.Context, key string) (KeyInfo, error)
	Lock(ctx context.Context, key string) error
	Unlock(ctx context.Context, key string) error
}

type KeyInfo struct {
	Key  string
	Size int64

	// Modified is the last modification time as a Unix timestamp (seconds).
	Modified int64

	// IsTerminal indicates whether this is a leaf node (i.e., a file) or a directory.
	IsTerminal bool
}

type Locker interface {
	Lock(ctx context.Context, name string) error
	Unlock(ctx context.Context, name string) error
}
