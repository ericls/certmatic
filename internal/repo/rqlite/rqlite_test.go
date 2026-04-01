package rqlite

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ericls/certmatic/internal/repo/repotest"
	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
)

// ---------- helpers ----------

func rqliteAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("TEST_RQLITE_ADDR")
	if addr == "" {
		addr = "http://localhost:4001?disableClusterDiscovery=true"
	}

	hostPort := addr
	for _, prefix := range []string{"http://", "https://"} {
		hostPort, _ = strings.CutPrefix(hostPort, prefix)
	}
	if i := strings.IndexByte(hostPort, '?'); i != -1 {
		hostPort = hostPort[:i]
	}
	if _, _, err := net.SplitHostPort(hostPort); err != nil {
		hostPort = hostPort + ":4001"
	}

	conn, err := net.DialTimeout("tcp", hostPort, 500*time.Millisecond)
	if err != nil {
		t.Skipf("rqlite not available at %s: %v", hostPort, err)
	}
	conn.Close()
	return addr
}

func cleanupTables(t *testing.T, sc *sharedConn) {
	t.Helper()
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.conn.Write([]string{
		"DELETE FROM sessions",
		"DELETE FROM domains",
	})
}

// ---------- shared suite ----------

func TestDomainRepo(t *testing.T) {
	repotest.RunDomainRepoTests(t, func(t *testing.T) domain.DomainRepo {
		t.Helper()
		addr := rqliteAddr(t)
		store, err := NewDomainStore(addr)
		if err != nil {
			t.Fatalf("NewDomainStore: %v", err)
		}
		cleanupTables(t, store.sc)
		t.Cleanup(func() { store.Destruct() })
		return store
	})
}

func TestSessionStore(t *testing.T) {
	repotest.RunSessionStoreTests(t, func(t *testing.T) pkgsession.SessionStore {
		t.Helper()
		addr := rqliteAddr(t)
		store, err := NewSessionStore(addr)
		if err != nil {
			t.Fatalf("NewSessionStore: %v", err)
		}
		cleanupTables(t, store.sc)
		t.Cleanup(func() { store.Destruct() })
		return store
	})
}

// ---------- backend-specific ----------

func TestDomainStore_UniqueID(t *testing.T) {
	addr := rqliteAddr(t)
	store, err := NewDomainStore(addr)
	if err != nil {
		t.Fatalf("NewDomainStore: %v", err)
	}
	cleanupTables(t, store.sc)
	t.Cleanup(func() { store.Destruct() })

	want := "rqlite:" + store.httpAddr
	if got := store.UniqueID(); got != want {
		t.Errorf("UniqueID() = %q, want %q", got, want)
	}
}
