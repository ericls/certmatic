package repotest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
)

func ptr[T any](v T) *T { return &v }

func signTestToken(key []byte, sessionID string) string {
	idB64 := base64.RawURLEncoding.EncodeToString([]byte(sessionID))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(idB64))
	sig := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return idB64 + "." + sigB64
}

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

// RunDomainRepoTests runs the standard suite of tests for any domain.DomainRepo implementation.
func RunDomainRepoTests(t *testing.T, newStore func(t *testing.T) domain.DomainRepo) {
	t.Run("Get_NotFound", func(t *testing.T) {
		store := newStore(t)
		_, err := store.Get(context.Background(), "nonexistent.com")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("SetAndGet", func(t *testing.T) {
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
				store := newStore(t)
				ctx := context.Background()

				d := tt.domain
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
	})

	t.Run("Set_Upsert", func(t *testing.T) {
		store := newStore(t)
		ctx := context.Background()

		d := &domain.Domain{Hostname: "upsert.com", TenantID: "old", OwnershipVerified: false}
		if err := store.Set(ctx, d); err != nil {
			t.Fatalf("Set: %v", err)
		}

		d.TenantID = "new"
		d.OwnershipVerified = true
		if err := store.Set(ctx, d); err != nil {
			t.Fatalf("Set (upsert): %v", err)
		}

		got, err := store.Get(ctx, "upsert.com")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Domain.TenantID != "new" {
			t.Errorf("TenantID = %q, want %q", got.Domain.TenantID, "new")
		}
		if !got.Domain.OwnershipVerified {
			t.Error("OwnershipVerified should be true after upsert")
		}
	})

	t.Run("Patch", func(t *testing.T) {
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
				store := newStore(t)
				ctx := context.Background()

				orig := &domain.Domain{Hostname: "patch.com", TenantID: "orig", VerificationToken: "orig-tok"}
				if err := store.Set(ctx, orig); err != nil {
					t.Fatalf("Set: %v", err)
				}

				if err := store.Patch(ctx, "patch.com", tt.patch); err != nil {
					t.Fatalf("Patch: %v", err)
				}

				got, err := store.Get(ctx, "patch.com")
				if err != nil {
					t.Fatalf("Get: %v", err)
				}
				tt.check(t, got.Domain)
			})
		}
	})

	t.Run("Patch_NotFound", func(t *testing.T) {
		store := newStore(t)
		err := store.Patch(context.Background(), "ghost.com", domain.DomainPatch{TenantID: ptr("x")})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store := newStore(t)
		ctx := context.Background()

		d := &domain.Domain{Hostname: "delete.com"}
		if err := store.Set(ctx, d); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := store.Delete(ctx, "delete.com"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		_, err := store.Get(ctx, "delete.com")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound after delete, got %v", err)
		}
	})
}

// RunSessionStoreTests runs the standard suite of tests for any session.SessionStore implementation.
func RunSessionStoreTests(t *testing.T, newStore func(t *testing.T) pkgsession.SessionStore) {
	t.Run("StoreAndGet", func(t *testing.T) {
		store := newStore(t)
		sess := testSession("sess-001", "example.com", time.Now().Add(1*time.Hour))

		if err := store.StoreSession(sess); err != nil {
			t.Fatalf("StoreSession: %v", err)
		}

		got, err := store.GetSession("sess-001")
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
	})

	t.Run("StoreAndGet_ProviderManaged", func(t *testing.T) {
		store := newStore(t)
		sess := &pkgsession.Session{
			SessionID:                 "sess-pm",
			Hostname:                  "pm.com",
			ExpiresAt:                 time.Now().Add(1 * time.Hour),
			OwnershipVerificationMode: pkgsession.OwnershipVerificationModeProviderManaged,
		}
		if err := store.StoreSession(sess); err != nil {
			t.Fatalf("StoreSession: %v", err)
		}
		got, err := store.GetSession("sess-pm")
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.OwnershipVerificationMode != pkgsession.OwnershipVerificationModeProviderManaged {
			t.Errorf("mode = %q, want provider_managed", got.OwnershipVerificationMode)
		}
	})

	t.Run("GetSession_NotFound", func(t *testing.T) {
		store := newStore(t)
		_, err := store.GetSession("nonexistent")
		if !errors.Is(err, pkgsession.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("GetSession_Expired", func(t *testing.T) {
		store := newStore(t)
		sess := testSession("sess-exp", "exp.com", time.Now().Add(-1*time.Hour))
		if err := store.StoreSession(sess); err != nil {
			t.Fatalf("StoreSession: %v", err)
		}
		_, err := store.GetSession("sess-exp")
		if !errors.Is(err, pkgsession.ErrExpiredToken) {
			t.Fatalf("expected ErrExpiredToken, got %v", err)
		}
	})

	t.Run("RedeemToken_Success", func(t *testing.T) {
		store := newStore(t)
		sess := testSession("sess-redeem", "redeem.com", time.Now().Add(1*time.Hour))
		if err := store.StoreSession(sess); err != nil {
			t.Fatalf("StoreSession: %v", err)
		}

		token := signTestToken(testSigningKey, "sess-redeem")
		got, err := store.RedeemToken(testSigningKey, token)
		if err != nil {
			t.Fatalf("RedeemToken: %v", err)
		}
		if got.SessionID != "sess-redeem" {
			t.Errorf("SessionID = %q, want %q", got.SessionID, "sess-redeem")
		}
	})

	t.Run("RedeemToken_Replay", func(t *testing.T) {
		store := newStore(t)
		sess := testSession("sess-replay", "replay.com", time.Now().Add(1*time.Hour))
		if err := store.StoreSession(sess); err != nil {
			t.Fatalf("StoreSession: %v", err)
		}

		token := signTestToken(testSigningKey, "sess-replay")

		if _, err := store.RedeemToken(testSigningKey, token); err != nil {
			t.Fatalf("first RedeemToken: %v", err)
		}

		_, err := store.RedeemToken(testSigningKey, token)
		if !errors.Is(err, pkgsession.ErrTokenReplayed) {
			t.Fatalf("expected ErrTokenReplayed, got %v", err)
		}
	})

	t.Run("RedeemToken_Expired", func(t *testing.T) {
		store := newStore(t)
		sess := testSession("sess-rexp", "rexp.com", time.Now().Add(-1*time.Hour))
		if err := store.StoreSession(sess); err != nil {
			t.Fatalf("StoreSession: %v", err)
		}

		token := signTestToken(testSigningKey, "sess-rexp")
		_, err := store.RedeemToken(testSigningKey, token)
		if !errors.Is(err, pkgsession.ErrExpiredToken) {
			t.Fatalf("expected ErrExpiredToken, got %v", err)
		}
	})

	t.Run("RedeemToken_InvalidToken", func(t *testing.T) {
		store := newStore(t)
		_, err := store.RedeemToken(testSigningKey, "garbage-token")
		if !errors.Is(err, pkgsession.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("RedeemToken_WrongKey", func(t *testing.T) {
		store := newStore(t)
		sess := testSession("sess-wk", "wk.com", time.Now().Add(1*time.Hour))
		if err := store.StoreSession(sess); err != nil {
			t.Fatalf("StoreSession: %v", err)
		}

		token := signTestToken([]byte("wrong-key-wrong-key-wrong-key!!!"), "sess-wk")
		_, err := store.RedeemToken(testSigningKey, token)
		if !errors.Is(err, pkgsession.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("ClearExpired", func(t *testing.T) {
		store := newStore(t)

		expired := testSession("sess-old", "old.com", time.Now().Add(-1*time.Hour))
		active := testSession("sess-new", "new.com", time.Now().Add(1*time.Hour))

		if err := store.StoreSession(expired); err != nil {
			t.Fatalf("StoreSession expired: %v", err)
		}
		if err := store.StoreSession(active); err != nil {
			t.Fatalf("StoreSession active: %v", err)
		}

		if err := store.ClearExpired(); err != nil {
			t.Fatalf("ClearExpired: %v", err)
		}

		if _, err := store.GetSession("sess-new"); err != nil {
			t.Fatalf("GetSession active after clear: %v", err)
		}

		_, err := store.GetSession("sess-old")
		if err == nil {
			t.Fatal("expected error for cleared expired session")
		}
	})
}
