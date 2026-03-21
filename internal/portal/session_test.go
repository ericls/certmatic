package portal

import (
	"testing"
	"time"
)

// --- Token signing/verification ---

func TestSignAndVerifySessionID_RoundTrip(t *testing.T) {
	key := []byte("test-signing-key")
	sessionID := "550e8400-e29b-41d4-a716-446655440000"

	token, err := signSessionID(key, sessionID)
	if err != nil {
		t.Fatalf("signSessionID failed: %v", err)
	}

	got, err := verifyTokenGetSessionID(key, token)
	if err != nil {
		t.Fatalf("verifyTokenGetSessionID failed: %v", err)
	}
	if got != sessionID {
		t.Errorf("expected session ID %q, got %q", sessionID, got)
	}
}

func TestVerifyTokenGetSessionID_TamperedSignature(t *testing.T) {
	key := []byte("test-signing-key")
	token, _ := signSessionID(key, "test-id")
	// Flip the last byte of the signature to tamper.
	tampered := token[:len(token)-1] + string([]byte{token[len(token)-1] ^ 0xff})

	_, err := verifyTokenGetSessionID(key, tampered)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifyTokenGetSessionID_WrongKey(t *testing.T) {
	token, _ := signSessionID([]byte("key-a"), "test-id")
	_, err := verifyTokenGetSessionID([]byte("key-b"), token)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifyTokenGetSessionID_NoDot(t *testing.T) {
	_, err := verifyTokenGetSessionID([]byte("key"), "nodothere")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifyTokenGetSessionID_InvalidBase64Signature(t *testing.T) {
	_, err := verifyTokenGetSessionID([]byte("key"), "validpart.!!!invalid!!!")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

// --- MemorySessionStore ---

func TestMemorySessionStore_StoreAndGet(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	session := &Session{
		SessionID: "test-session-id",
		Hostname:  "example.com",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := store.StoreSession(session); err != nil {
		t.Fatalf("StoreSession failed: %v", err)
	}

	got, err := store.GetSession(session.SessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.Hostname != "example.com" {
		t.Errorf("expected hostname %q, got %q", "example.com", got.Hostname)
	}
}

func TestMemorySessionStore_GetSession_NotFound(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	_, err := store.GetSession("nonexistent")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestMemorySessionStore_GetSession_Expired(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	session := &Session{
		SessionID: "expired-session",
		Hostname:  "example.com",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	store.StoreSession(session)

	_, err := store.GetSession("expired-session")
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestMemorySessionStore_RedeemToken_Success(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	key := []byte("signing-key")
	token, _, err := CreateToken(store, key, "example.com", time.Hour, "", "", OwnershipVerificationModeDNSChallenge, "", "")
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	session, err := store.RedeemToken(key, token)
	if err != nil {
		t.Fatalf("RedeemToken failed: %v", err)
	}
	if session.Hostname != "example.com" {
		t.Errorf("expected hostname %q, got %q", "example.com", session.Hostname)
	}
}

func TestMemorySessionStore_RedeemToken_Replay(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	key := []byte("signing-key")
	token, _, _ := CreateToken(store, key, "example.com", time.Hour, "", "", OwnershipVerificationModeDNSChallenge, "", "")

	store.RedeemToken(key, token) // first use
	_, err := store.RedeemToken(key, token)
	if err != ErrTokenReplayed {
		t.Errorf("expected ErrTokenReplayed, got %v", err)
	}
}

func TestMemorySessionStore_RedeemToken_Expired(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	key := []byte("signing-key")
	token, _, _ := CreateToken(store, key, "example.com", -time.Second, "", "", OwnershipVerificationModeDNSChallenge, "", "")

	_, err := store.RedeemToken(key, token)
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestMemorySessionStore_RedeemToken_InvalidToken(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	_, err := store.RedeemToken([]byte("key"), "invalid.token.that.doesnt.exist")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestMemorySessionStore_ClearExpired(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	store.StoreSession(&Session{SessionID: "expired", Hostname: "a.com", ExpiresAt: time.Now().Add(-time.Hour)})
	store.StoreSession(&Session{SessionID: "active", Hostname: "b.com", ExpiresAt: time.Now().Add(time.Hour)})

	store.ClearExpired()

	if _, err := store.GetSession("expired"); err == nil {
		t.Error("expected expired session to be removed after ClearExpired")
	}
	if _, err := store.GetSession("active"); err != nil {
		t.Errorf("expected active session to remain: %v", err)
	}
}

// --- CreateToken full round-trip ---

func TestCreateToken_SessionContents(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Destruct()

	key := []byte("round-trip-key")
	token, expiresAt, err := CreateToken(
		store, key,
		"sub.example.com",
		time.Hour,
		"https://app.example.com/back",
		"Back",
		OwnershipVerificationModeProviderManaged,
		"https://app.example.com/verify",
		"Verify Ownership",
	)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	if expiresAt.IsZero() {
		t.Error("expected non-zero expiresAt")
	}

	session, err := store.RedeemToken(key, token)
	if err != nil {
		t.Fatalf("RedeemToken failed: %v", err)
	}
	if session.Hostname != "sub.example.com" {
		t.Errorf("expected hostname %q, got %q", "sub.example.com", session.Hostname)
	}
	if session.BackURL != "https://app.example.com/back" {
		t.Errorf("expected back URL %q, got %q", "https://app.example.com/back", session.BackURL)
	}
	if session.OwnershipVerificationMode != OwnershipVerificationModeProviderManaged {
		t.Errorf("unexpected ownership mode %q", session.OwnershipVerificationMode)
	}
	if session.VerifyOwnershipURL != "https://app.example.com/verify" {
		t.Errorf("unexpected verify URL %q", session.VerifyOwnershipURL)
	}
}
