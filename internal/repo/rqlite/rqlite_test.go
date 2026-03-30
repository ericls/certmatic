package rqlite

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
)

// ---------- helpers ----------

func rqliteAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("RQLITE_ADDR")
	if addr == "" {
		// Disable cluster discovery by default so tests work with a single
		// Docker container where the container's internal hostname isn't
		// reachable from the host.
		addr = "http://localhost:4001?disableClusterDiscovery=true"
	}

	// Derive a plain host:port for the TCP reachability check.
	hostPort := addr
	for _, prefix := range []string{"http://", "https://"} {
		hostPort, _ = strings.CutPrefix(hostPort, prefix)
	}
	// Strip query string for the dial.
	if i := strings.IndexByte(hostPort, '?'); i != -1 {
		hostPort = hostPort[:i]
	}
	// Ensure there's a port; default to 4001.
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

// cleanupTables drops test data between tests. Since rqlite is shared,
// we clear tables rather than creating isolated databases.
func cleanupTables(t *testing.T, sc *sharedConn) {
	t.Helper()
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.conn.Write([]string{
		"DELETE FROM sessions",
		"DELETE FROM domains",
	})
}

func newTestDomainStore(t *testing.T) *DomainStore {
	t.Helper()
	addr := rqliteAddr(t)
	store, err := NewDomainStore(addr)
	if err != nil {
		t.Fatalf("NewDomainStore: %v", err)
	}
	cleanupTables(t, store.sc)
	t.Cleanup(func() { store.Destruct() })
	return store
}

func newTestSessionStore(t *testing.T) *SessionStore {
	t.Helper()
	addr := rqliteAddr(t)
	store, err := NewSessionStore(addr)
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	cleanupTables(t, store.sc)
	t.Cleanup(func() { store.Destruct() })
	return store
}

func signTestToken(key []byte, sessionID string) string {
	idB64 := base64.RawURLEncoding.EncodeToString([]byte(sessionID))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(idB64))
	sig := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return idB64 + "." + sigB64
}

func ptr[T any](v T) *T { return &v }

// uniqueHostname generates a unique hostname per test to avoid cross-test collisions.
func uniqueHostname(t *testing.T, base string) string {
	return fmt.Sprintf("%s-%d.test", base, time.Now().UnixNano())
}

// ---------- DomainStore ----------

func TestDomainStore_Get_NotFound(t *testing.T) {
	store := newTestDomainStore(t)
	_, err := store.Get(context.Background(), "nonexistent.com")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDomainStore_SetAndGet(t *testing.T) {
	tests := []struct {
		name   string
		domain domain.Domain
	}{
		{
			name: "all fields",
			domain: domain.Domain{
				Hostname:          "example.com",
				TenantID:          "tenant-1",
				OwnershipVerified: true,
				VerificationToken: "tok-abc",
			},
		},
		{
			name: "minimal fields",
			domain: domain.Domain{
				Hostname: "minimal.com",
			},
		},
		{
			name: "not verified",
			domain: domain.Domain{
				Hostname:          "unverified.com",
				TenantID:          "tenant-2",
				OwnershipVerified: false,
				VerificationToken: "tok-xyz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestDomainStore(t)
			ctx := context.Background()

			d := tt.domain
			d.Hostname = uniqueHostname(t, tt.domain.Hostname)

			if err := store.Set(ctx, &d); err != nil {
				t.Fatalf("Set: %v", err)
			}

			got, err := store.Get(ctx, d.Hostname)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}

			if got.Domain.Hostname != d.Hostname {
				t.Errorf("Hostname = %q, want %q", got.Domain.Hostname, d.Hostname)
			}
			if got.Domain.TenantID != d.TenantID {
				t.Errorf("TenantID = %q, want %q", got.Domain.TenantID, d.TenantID)
			}
			if got.Domain.OwnershipVerified != d.OwnershipVerified {
				t.Errorf("OwnershipVerified = %v, want %v",
					got.Domain.OwnershipVerified, d.OwnershipVerified)
			}
			if got.Domain.VerificationToken != d.VerificationToken {
				t.Errorf("VerificationToken = %q, want %q",
					got.Domain.VerificationToken, d.VerificationToken)
			}
			if got.Source != store.UniqueID() {
				t.Errorf("Source = %q, want %q", got.Source, store.UniqueID())
			}
			if time.Since(got.CachedAt) > 5*time.Second {
				t.Errorf("CachedAt too old: %v", got.CachedAt)
			}
		})
	}
}

func TestDomainStore_Set_Upsert(t *testing.T) {
	store := newTestDomainStore(t)
	ctx := context.Background()

	hostname := uniqueHostname(t, "upsert")
	d := &domain.Domain{Hostname: hostname, TenantID: "old", OwnershipVerified: false}
	if err := store.Set(ctx, d); err != nil {
		t.Fatalf("Set: %v", err)
	}

	d.TenantID = "new"
	d.OwnershipVerified = true
	if err := store.Set(ctx, d); err != nil {
		t.Fatalf("Set (upsert): %v", err)
	}

	got, err := store.Get(ctx, hostname)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Domain.TenantID != "new" {
		t.Errorf("TenantID = %q, want %q", got.Domain.TenantID, "new")
	}
	if !got.Domain.OwnershipVerified {
		t.Error("OwnershipVerified should be true after upsert")
	}
}

func TestDomainStore_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch domain.DomainPatch
		check func(t *testing.T, d *domain.Domain)
	}{
		{
			name:  "patch tenant only",
			patch: domain.DomainPatch{TenantID: ptr("patched-tenant")},
			check: func(t *testing.T, d *domain.Domain) {
				if d.TenantID != "patched-tenant" {
					t.Errorf("TenantID = %q, want %q", d.TenantID, "patched-tenant")
				}
				if d.OwnershipVerified != false {
					t.Error("OwnershipVerified should remain false")
				}
			},
		},
		{
			name:  "patch ownership only",
			patch: domain.DomainPatch{OwnershipVerified: ptr(true)},
			check: func(t *testing.T, d *domain.Domain) {
				if !d.OwnershipVerified {
					t.Error("OwnershipVerified should be true")
				}
				if d.TenantID != "orig" {
					t.Errorf("TenantID should remain %q, got %q", "orig", d.TenantID)
				}
			},
		},
		{
			name:  "patch token only",
			patch: domain.DomainPatch{VerificationToken: ptr("new-tok")},
			check: func(t *testing.T, d *domain.Domain) {
				if d.VerificationToken != "new-tok" {
					t.Errorf("VerificationToken = %q, want %q", d.VerificationToken, "new-tok")
				}
			},
		},
		{
			name: "patch all fields",
			patch: domain.DomainPatch{
				TenantID:          ptr("all-tenant"),
				OwnershipVerified: ptr(true),
				VerificationToken: ptr("all-tok"),
			},
			check: func(t *testing.T, d *domain.Domain) {
				if d.TenantID != "all-tenant" {
					t.Errorf("TenantID = %q", d.TenantID)
				}
				if !d.OwnershipVerified {
					t.Error("OwnershipVerified should be true")
				}
				if d.VerificationToken != "all-tok" {
					t.Errorf("VerificationToken = %q", d.VerificationToken)
				}
			},
		},
		{
			name:  "empty patch",
			patch: domain.DomainPatch{},
			check: func(t *testing.T, d *domain.Domain) {
				if d.TenantID != "orig" {
					t.Errorf("TenantID should remain %q", "orig")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestDomainStore(t)
			ctx := context.Background()

			hostname := uniqueHostname(t, "patch")
			orig := &domain.Domain{Hostname: hostname, TenantID: "orig", VerificationToken: "orig-tok"}
			if err := store.Set(ctx, orig); err != nil {
				t.Fatalf("Set: %v", err)
			}

			if err := store.Patch(ctx, hostname, tt.patch); err != nil {
				t.Fatalf("Patch: %v", err)
			}

			got, err := store.Get(ctx, hostname)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			tt.check(t, got.Domain)
		})
	}
}

func TestDomainStore_Patch_NotFound(t *testing.T) {
	store := newTestDomainStore(t)
	err := store.Patch(context.Background(), "ghost.com", domain.DomainPatch{TenantID: ptr("x")})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDomainStore_Delete(t *testing.T) {
	store := newTestDomainStore(t)
	ctx := context.Background()

	hostname := uniqueHostname(t, "delete")
	d := &domain.Domain{Hostname: hostname}
	if err := store.Set(ctx, d); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.Delete(ctx, hostname); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(ctx, hostname)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDomainStore_UniqueID(t *testing.T) {
	store := newTestDomainStore(t)
	want := "rqlite:" + store.httpAddr
	if got := store.UniqueID(); got != want {
		t.Errorf("UniqueID() = %q, want %q", got, want)
	}
}

// ---------- SessionStore ----------

var testSigningKey = []byte("test-signing-key-32bytes-long!!!")

func testSession(sessionID, hostname string, expiresAt time.Time) *pkgsession.Session {
	return &pkgsession.Session{
		SessionID:                 sessionID,
		Hostname:                  hostname,
		ExpiresAt:                 expiresAt,
		BackURL:                   "https://example.com/back",
		BackText:                  "Go Back",
		OwnershipVerificationMode: pkgsession.OwnershipVerificationModeDNSChallenge,
		VerifyOwnershipURL:        "https://example.com/verify",
		VerifyOwnershipText:       "Verify",
	}
}

func TestSessionStore_StoreAndGet(t *testing.T) {
	store := newTestSessionStore(t)
	sessID := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	sess := testSession(sessID, "example.com", time.Now().Add(1*time.Hour))

	if err := store.StoreSession(sess); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}

	got, err := store.GetSession(sessID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if got.SessionID != sess.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, sess.SessionID)
	}
	if got.Hostname != sess.Hostname {
		t.Errorf("Hostname = %q, want %q", got.Hostname, sess.Hostname)
	}
	if got.BackURL != sess.BackURL {
		t.Errorf("BackURL = %q, want %q", got.BackURL, sess.BackURL)
	}
	if got.BackText != sess.BackText {
		t.Errorf("BackText = %q, want %q", got.BackText, sess.BackText)
	}
	if got.OwnershipVerificationMode != sess.OwnershipVerificationMode {
		t.Errorf("OwnershipVerificationMode = %q, want %q", got.OwnershipVerificationMode, sess.OwnershipVerificationMode)
	}
	if got.VerifyOwnershipURL != sess.VerifyOwnershipURL {
		t.Errorf("VerifyOwnershipURL = %q, want %q", got.VerifyOwnershipURL, sess.VerifyOwnershipURL)
	}
	if got.VerifyOwnershipText != sess.VerifyOwnershipText {
		t.Errorf("VerifyOwnershipText = %q, want %q", got.VerifyOwnershipText, sess.VerifyOwnershipText)
	}
}

func TestSessionStore_GetSession_NotFound(t *testing.T) {
	store := newTestSessionStore(t)
	_, err := store.GetSession("nonexistent")
	if !errors.Is(err, pkgsession.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSessionStore_GetSession_Expired(t *testing.T) {
	store := newTestSessionStore(t)
	sessID := fmt.Sprintf("sess-exp-%d", time.Now().UnixNano())
	sess := testSession(sessID, "exp.com", time.Now().Add(-1*time.Hour))
	if err := store.StoreSession(sess); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}
	_, err := store.GetSession(sessID)
	if !errors.Is(err, pkgsession.ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestSessionStore_RedeemToken_Success(t *testing.T) {
	store := newTestSessionStore(t)
	sessID := fmt.Sprintf("sess-redeem-%d", time.Now().UnixNano())
	sess := testSession(sessID, "redeem.com", time.Now().Add(1*time.Hour))
	if err := store.StoreSession(sess); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}

	token := signTestToken(testSigningKey, sessID)
	got, err := store.RedeemToken(testSigningKey, token)
	if err != nil {
		t.Fatalf("RedeemToken: %v", err)
	}
	if got.SessionID != sessID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, sessID)
	}
}

func TestSessionStore_RedeemToken_Replay(t *testing.T) {
	store := newTestSessionStore(t)
	sessID := fmt.Sprintf("sess-replay-%d", time.Now().UnixNano())
	sess := testSession(sessID, "replay.com", time.Now().Add(1*time.Hour))
	if err := store.StoreSession(sess); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}

	token := signTestToken(testSigningKey, sessID)

	// First redemption succeeds.
	if _, err := store.RedeemToken(testSigningKey, token); err != nil {
		t.Fatalf("first RedeemToken: %v", err)
	}

	// Second redemption fails.
	_, err := store.RedeemToken(testSigningKey, token)
	if !errors.Is(err, pkgsession.ErrTokenReplayed) {
		t.Fatalf("expected ErrTokenReplayed, got %v", err)
	}
}

func TestSessionStore_RedeemToken_Expired(t *testing.T) {
	store := newTestSessionStore(t)
	sessID := fmt.Sprintf("sess-rexp-%d", time.Now().UnixNano())
	sess := testSession(sessID, "rexp.com", time.Now().Add(-1*time.Hour))
	if err := store.StoreSession(sess); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}

	token := signTestToken(testSigningKey, sessID)
	_, err := store.RedeemToken(testSigningKey, token)
	if !errors.Is(err, pkgsession.ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestSessionStore_RedeemToken_InvalidToken(t *testing.T) {
	store := newTestSessionStore(t)
	_, err := store.RedeemToken(testSigningKey, "garbage-token")
	if !errors.Is(err, pkgsession.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSessionStore_RedeemToken_WrongKey(t *testing.T) {
	store := newTestSessionStore(t)
	sessID := fmt.Sprintf("sess-wk-%d", time.Now().UnixNano())
	sess := testSession(sessID, "wk.com", time.Now().Add(1*time.Hour))
	if err := store.StoreSession(sess); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}

	token := signTestToken([]byte("wrong-key-wrong-key-wrong-key!!!"), sessID)
	_, err := store.RedeemToken(testSigningKey, token)
	if !errors.Is(err, pkgsession.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSessionStore_ClearExpired(t *testing.T) {
	store := newTestSessionStore(t)

	activeSessID := fmt.Sprintf("sess-new-%d", time.Now().UnixNano())
	expiredSessID := fmt.Sprintf("sess-old-%d", time.Now().UnixNano())

	expired := testSession(expiredSessID, "old.com", time.Now().Add(-1*time.Hour))
	active := testSession(activeSessID, "new.com", time.Now().Add(1*time.Hour))

	if err := store.StoreSession(expired); err != nil {
		t.Fatalf("StoreSession expired: %v", err)
	}
	if err := store.StoreSession(active); err != nil {
		t.Fatalf("StoreSession active: %v", err)
	}

	if err := store.ClearExpired(); err != nil {
		t.Fatalf("ClearExpired: %v", err)
	}

	// Active session should still work.
	if _, err := store.GetSession(activeSessID); err != nil {
		t.Fatalf("GetSession active after clear: %v", err)
	}

	// Expired session should be gone.
	_, err := store.GetSession(expiredSessID)
	if err == nil {
		t.Fatal("expected error for cleared expired session")
	}
}
