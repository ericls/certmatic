package domain

import (
	"context"
	"time"
)

type StoredDomain struct {
	Domain   *Domain       `json:"domain" yaml:"domain"`
	TTL      time.Duration `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	CachedAt time.Time     `json:"cached_at,omitempty" yaml:"cached_at,omitempty"`
	Source   string        `json:"source,omitempty" yaml:"source,omitempty"`
}

type DomainPatch struct {
	TenantID          *string
	OwnershipVerified *bool
	VerificationToken *string
}

type DomainRepo interface {
	UniqueID() string
	Get(ctx context.Context, hostname string) (*StoredDomain, error)
	Set(ctx context.Context, domain *Domain) error
	Patch(ctx context.Context, hostname string, patch DomainPatch) error
	Delete(ctx context.Context, hostname string) error
	// Invalidate(ctx context.Context, hostname string) error

	Destruct() error
}

type ReadOnlyDomainRepo interface {
	DomainRepo
	ReadOnly() bool
}

type CachingDomainRepo interface {
	DomainRepo

	// SetWithTTL stores a domain with a specific TTL override.
	SetWithTTL(ctx context.Context, domain *Domain, ttl time.Duration) error

	// Clear removes all entries from the cache.
	Clear(ctx context.Context) error
}
