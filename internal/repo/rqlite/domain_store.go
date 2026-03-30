package rqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	gorqlite "github.com/rqlite/gorqlite"

	"github.com/ericls/certmatic/pkg/domain"
)

// DomainStore is an rqlite-backed domain.DomainRepo.
type DomainStore struct {
	httpAddr string
	sc       *sharedConn
}

// NewDomainStore opens (or reuses) the rqlite connection at httpAddr and returns a DomainStore.
func NewDomainStore(httpAddr string) (*DomainStore, error) {
	sc, err := acquireConn(httpAddr)
	if err != nil {
		return nil, err
	}
	return &DomainStore{httpAddr: httpAddr, sc: sc}, nil
}

// Destruct implements caddy.Destructor — releases the shared connection.
func (s *DomainStore) Destruct() error {
	releaseConn(s.httpAddr)
	return nil
}

// UniqueID implements domain.DomainRepo.
func (s *DomainStore) UniqueID() string {
	return "rqlite:" + s.httpAddr
}

// Get implements domain.DomainRepo.
func (s *DomainStore) Get(_ context.Context, hostname string) (*domain.StoredDomain, error) {
	s.sc.mu.Lock()
	defer s.sc.mu.Unlock()

	qr, err := s.sc.conn.QueryOneParameterized(gorqlite.ParameterizedStatement{
		Query:     `SELECT hostname, tenant_id, ownership_verified, verification_token FROM domains WHERE hostname = ?`,
		Arguments: []any{hostname},
	})
	if err != nil {
		return nil, fmt.Errorf("get domain %q: %w", hostname, err)
	}
	if qr.Err != nil {
		return nil, fmt.Errorf("get domain %q: %w", hostname, qr.Err)
	}

	if !qr.Next() {
		return nil, domain.ErrNotFound
	}

	var d domain.Domain
	var ownershipVerified int
	if err := qr.Scan(&d.Hostname, &d.TenantID, &ownershipVerified, &d.VerificationToken); err != nil {
		return nil, fmt.Errorf("get domain %q: %w", hostname, err)
	}
	d.OwnershipVerified = ownershipVerified != 0

	return &domain.StoredDomain{
		Domain:   &d,
		CachedAt: time.Now(),
		Source:   s.UniqueID(),
	}, nil
}

// Set implements domain.DomainRepo.
func (s *DomainStore) Set(_ context.Context, d *domain.Domain) error {
	s.sc.mu.Lock()
	defer s.sc.mu.Unlock()

	wr, err := s.sc.conn.WriteOneParameterized(gorqlite.ParameterizedStatement{
		Query: `INSERT INTO domains (hostname, tenant_id, ownership_verified, verification_token, updated_at)
			 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(hostname) DO UPDATE SET
			   tenant_id = excluded.tenant_id,
			   ownership_verified = excluded.ownership_verified,
			   verification_token = excluded.verification_token,
			   updated_at = CURRENT_TIMESTAMP`,
		Arguments: []any{d.Hostname, d.TenantID, boolToInt(d.OwnershipVerified), d.VerificationToken},
	})
	if err != nil {
		return fmt.Errorf("set domain %q: %w", d.Hostname, err)
	}
	if wr.Err != nil {
		return fmt.Errorf("set domain %q: %w", d.Hostname, wr.Err)
	}
	return nil
}

// Patch implements domain.DomainRepo.
func (s *DomainStore) Patch(_ context.Context, hostname string, patch domain.DomainPatch) error {
	s.sc.mu.Lock()
	defer s.sc.mu.Unlock()

	setClauses := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []any{}

	if patch.TenantID != nil {
		setClauses = append(setClauses, "tenant_id = ?")
		args = append(args, *patch.TenantID)
	}
	if patch.OwnershipVerified != nil {
		setClauses = append(setClauses, "ownership_verified = ?")
		args = append(args, boolToInt(*patch.OwnershipVerified))
	}
	if patch.VerificationToken != nil {
		setClauses = append(setClauses, "verification_token = ?")
		args = append(args, *patch.VerificationToken)
	}

	args = append(args, hostname)
	query := fmt.Sprintf("UPDATE domains SET %s WHERE hostname = ?",
		strings.Join(setClauses, ", "))

	wr, err := s.sc.conn.WriteOneParameterized(gorqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	})
	if err != nil {
		return fmt.Errorf("patch domain %q: %w", hostname, err)
	}
	if wr.Err != nil {
		return fmt.Errorf("patch domain %q: %w", hostname, wr.Err)
	}
	if wr.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete implements domain.DomainRepo.
func (s *DomainStore) Delete(_ context.Context, hostname string) error {
	s.sc.mu.Lock()
	defer s.sc.mu.Unlock()

	wr, err := s.sc.conn.WriteOneParameterized(gorqlite.ParameterizedStatement{
		Query:     `DELETE FROM domains WHERE hostname = ?`,
		Arguments: []any{hostname},
	})
	if err != nil {
		return fmt.Errorf("delete domain %q: %w", hostname, err)
	}
	if wr.Err != nil {
		return fmt.Errorf("delete domain %q: %w", hostname, wr.Err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var _ domain.DomainRepo = (*DomainStore)(nil)
