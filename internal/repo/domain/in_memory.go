package domain

import (
	"context"
	"sync"
	"time"

	domain "github.com/ericls/certmatic/pkg/domain"
)

type InMemoryDomainRepo struct {
	mu      sync.RWMutex
	domains map[string]*domain.StoredDomain
	name    string
}

func NewInMemoryDomainRepo(name string) *InMemoryDomainRepo {
	return &InMemoryDomainRepo{
		domains: make(map[string]*domain.StoredDomain),
		name:    name,
	}
}

// UniqueID returns a unique identifier for this repository instance.
func (repo *InMemoryDomainRepo) UniqueID() string {
	return repo.name
}

// Get retrieves a domain by hostname.
func (repo *InMemoryDomainRepo) Get(ctx context.Context, hostname string) (*domain.StoredDomain, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	stored, exists := repo.domains[hostname]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return &domain.StoredDomain{
		Domain:   stored.Domain.Clone(),
		TTL:      stored.TTL,
		CachedAt: stored.CachedAt,
		Source:   stored.Source,
	}, nil
}

// Set stores a domain.
func (repo *InMemoryDomainRepo) Set(ctx context.Context, d *domain.Domain) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.domains[d.Hostname] = &domain.StoredDomain{
		Domain:   d.Clone(),
		CachedAt: time.Now(),
		Source:   repo.name,
	}
	return nil
}

// Delete removes a domain by hostname.
func (repo *InMemoryDomainRepo) Delete(ctx context.Context, hostname string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, exists := repo.domains[hostname]; !exists {
		return domain.ErrNotFound
	}
	delete(repo.domains, hostname)
	return nil
}

// Invalidate marks a domain as invalid/stale by removing it.
// func (repo *InMemoryDomainRepo) Invalidate(ctx context.Context, hostname string) error {
// 	repo.mu.Lock()
// 	defer repo.mu.Unlock()

// 	delete(repo.domains, hostname)
// 	return nil
// }

// Destruct closes the repo and releases any resources. For in-memory repo, there's nothing to do.
func (repo *InMemoryDomainRepo) Destruct() error {
	return nil
}
