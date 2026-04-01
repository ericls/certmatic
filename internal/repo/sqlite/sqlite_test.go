package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/ericls/certmatic/internal/repo/repotest"
	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
)

// ---------- shared suite ----------

func TestDomainRepo(t *testing.T) {
	repotest.RunDomainRepoTests(t, func(t *testing.T) domain.DomainRepo {
		t.Helper()
		path := filepath.Join(t.TempDir(), "test.db")
		store, err := NewDomainStore(path)
		if err != nil {
			t.Fatalf("NewDomainStore: %v", err)
		}
		t.Cleanup(func() { store.Destruct() })
		return store
	})
}

func TestSessionStore(t *testing.T) {
	repotest.RunSessionStoreTests(t, func(t *testing.T) pkgsession.SessionStore {
		t.Helper()
		path := filepath.Join(t.TempDir(), "test.db")
		store, err := NewSessionStore(path)
		if err != nil {
			t.Fatalf("NewSessionStore: %v", err)
		}
		t.Cleanup(func() { store.Destruct() })
		return store
	})
}

// ---------- backend-specific ----------

func TestDomainStore_SharedDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")

	store1, err := NewDomainStore(path)
	if err != nil {
		t.Fatalf("NewDomainStore store1: %v", err)
	}
	t.Cleanup(func() { store1.Destruct() })

	store2, err := NewDomainStore(path)
	if err != nil {
		t.Fatalf("NewDomainStore store2: %v", err)
	}
	t.Cleanup(func() { store2.Destruct() })

	ctx := t.Context()
	if err := store1.Set(ctx, &domain.Domain{Hostname: "shared.com", TenantID: "t1"}); err != nil {
		t.Fatalf("Set via store1: %v", err)
	}

	got, err := store2.Get(ctx, "shared.com")
	if err != nil {
		t.Fatalf("Get via store2: %v", err)
	}
	if got.Domain.TenantID != "t1" {
		t.Errorf("TenantID = %q, want %q", got.Domain.TenantID, "t1")
	}
}

func TestDomainStore_UniqueID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uid.db")
	store, err := NewDomainStore(path)
	if err != nil {
		t.Fatalf("NewDomainStore: %v", err)
	}
	t.Cleanup(func() { store.Destruct() })

	want := "sqlite:" + path
	if got := store.UniqueID(); got != want {
		t.Errorf("UniqueID() = %q, want %q", got, want)
	}
}

// ---------- db.go ----------

func TestAcquireDB_Migrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migrate.db")
	db, err := acquireDB(path)
	if err != nil {
		t.Fatalf("acquireDB: %v", err)
	}
	defer releaseDB(path)

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count == 0 {
		t.Error("expected at least one migration recorded")
	}

	if _, err := db.Exec("SELECT 1 FROM domains LIMIT 0"); err != nil {
		t.Fatalf("domains table not found: %v", err)
	}

	if _, err := db.Exec("SELECT 1 FROM sessions LIMIT 0"); err != nil {
		t.Fatalf("sessions table not found: %v", err)
	}
}

func TestAcquireDB_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idem.db")

	db1, err := acquireDB(path)
	if err != nil {
		t.Fatalf("first acquireDB: %v", err)
	}

	db2, err := acquireDB(path)
	if err != nil {
		t.Fatalf("second acquireDB: %v", err)
	}

	if db1 != db2 {
		t.Error("expected same *sql.DB pointer for same path")
	}

	releaseDB(path)
	releaseDB(path)
}
