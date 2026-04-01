package session

import (
	"testing"

	"github.com/ericls/certmatic/internal/repo/repotest"
	pkgsession "github.com/ericls/certmatic/pkg/session"
)

func TestMemorySessionStore(t *testing.T) {
	repotest.RunSessionStoreTests(t, func(t *testing.T) pkgsession.SessionStore {
		store := NewMemorySessionStore()
		t.Cleanup(func() { store.Destruct() })
		return store
	})
}
