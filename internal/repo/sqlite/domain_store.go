package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ericls/certmatic/pkg/domain"
)

// DomainStore is a SQLite-backed domain.DomainRepo.
// Multiple DomainStore instances pointing to the same file share a single *sql.DB.
type DomainStore struct {
	filePath string
	db       *sql.DB
}

// NewDomainStore opens (or reuses) the SQLite database at filePath and returns a DomainStore.
func NewDomainStore(filePath string) (*DomainStore, error) {
	db, err := acquireDB(filePath)
	if err != nil {
		return nil, err
	}
	return &DomainStore{filePath: filePath, db: db}, nil
}

// Destruct implements caddy.Destructor — releases the shared DB connection.
func (s *DomainStore) Destruct() error {
	releaseDB(s.filePath)
	return nil
}

// UniqueID implements domain.DomainRepo.
func (s *DomainStore) UniqueID() string {
	return "sqlite:" + s.filePath
}

// Get implements domain.DomainRepo.
func (s *DomainStore) Get(ctx context.Context, hostname string) (*domain.StoredDomain, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT hostname, tenant_id, ownership_verified, verification_token FROM domains WHERE hostname = ?`,
		hostname,
	)

	var d domain.Domain
	var ownershipVerified int
	err := row.Scan(&d.Hostname, &d.TenantID, &ownershipVerified, &d.VerificationToken)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
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
func (s *DomainStore) Set(ctx context.Context, d *domain.Domain) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO domains (hostname, tenant_id, ownership_verified, verification_token, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(hostname) DO UPDATE SET
		   tenant_id = excluded.tenant_id,
		   ownership_verified = excluded.ownership_verified,
		   verification_token = excluded.verification_token,
		   updated_at = CURRENT_TIMESTAMP`,
		d.Hostname, d.TenantID, boolToInt(d.OwnershipVerified), d.VerificationToken,
	)
	if err != nil {
		return fmt.Errorf("set domain %q: %w", d.Hostname, err)
	}
	return nil
}

// Patch implements domain.DomainRepo.
func (s *DomainStore) Patch(ctx context.Context, hostname string, patch domain.DomainPatch) error {
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

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("patch domain %q: %w", hostname, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete implements domain.DomainRepo.
func (s *DomainStore) Delete(ctx context.Context, hostname string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM domains WHERE hostname = ?`, hostname)
	if err != nil {
		return fmt.Errorf("delete domain %q: %w", hostname, err)
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
