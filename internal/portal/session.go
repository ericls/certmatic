package portal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrExpiredToken  = errors.New("token expired")
	ErrTokenReplayed = errors.New("token already used")
)

// OwnershipVerificationMode controls what the portal shows the user for ownership verification.
type OwnershipVerificationMode string

const (
	// OwnershipVerificationModeDNSChallenge instructs the portal to show a DNS TXT record that
	// the user must add to prove ownership. When the user runs Setup Check, the portal performs
	// a live DNS lookup against _certmatic-verify.{hostname} and automatically sets
	// ownership_verified=true on the domain if the record matches.
	OwnershipVerificationModeDNSChallenge OwnershipVerificationMode = "dns_challenge"

	// OwnershipVerificationModeProviderManaged indicates that an external SaaS/provider controls
	// verification. The portal shows a configurable "Verify Ownership" button linking to the
	// provider dashboard. The provider (or admin) calls ownership_verified=true on the admin API.
	OwnershipVerificationModeProviderManaged OwnershipVerificationMode = "provider_managed"
)

// Session represents an authenticated portal session scoped to a single hostname.
type Session struct {
	SessionID                 string
	Hostname                  string
	ExpiresAt                 time.Time
	BackURL                   string
	BackText                  string
	OwnershipVerificationMode OwnershipVerificationMode
	VerifyOwnershipURL        string
	VerifyOwnershipText       string
}

// SessionStore manages portal sessions.
type SessionStore interface {
	// StoreSession persists a newly created session.
	StoreSession(session *Session) error
	// RedeemToken validates an HMAC-signed token (one-time use) and returns the stored session.
	RedeemToken(signingKey []byte, token string) (*Session, error)
	// GetSession looks up an active session by session ID.
	GetSession(sessionID string) (*Session, error)
	// ClearExpired removes all sessions that have passed their expiry time.
	ClearExpired() error
}

// CreateToken stores a new session and returns an HMAC-signed token containing only the session ID.
func CreateToken(store SessionStore, signingKey []byte, hostname string, ttl time.Duration,
	backURL, backText string, ownershipMode OwnershipVerificationMode,
	verifyOwnershipURL, verifyOwnershipText string,
) (string, time.Time, error) {
	sessionID, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate session id: %w", err)
	}
	expiresAt := time.Now().UTC().Add(ttl)
	session := &Session{
		SessionID:                 sessionID.String(),
		Hostname:                  hostname,
		ExpiresAt:                 expiresAt,
		BackURL:                   backURL,
		BackText:                  backText,
		OwnershipVerificationMode: ownershipMode,
		VerifyOwnershipURL:        verifyOwnershipURL,
		VerifyOwnershipText:       verifyOwnershipText,
	}
	if err := store.StoreSession(session); err != nil {
		return "", time.Time{}, fmt.Errorf("store session: %w", err)
	}
	token, err := signSessionID(signingKey, sessionID.String())
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// signSessionID produces base64url(sessionID) + "." + base64url(HMAC-SHA256(key, base64url(sessionID))).
func signSessionID(signingKey []byte, sessionID string) (string, error) {
	idB64 := base64.RawURLEncoding.EncodeToString([]byte(sessionID))
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(idB64))
	sig := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return idB64 + "." + sigB64, nil
}

// verifyTokenGetSessionID verifies the HMAC signature and returns the session ID.
func verifyTokenGetSessionID(signingKey []byte, token string) (string, error) {
	dotIdx := -1
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			dotIdx = i
			break
		}
	}
	if dotIdx < 0 {
		return "", ErrInvalidToken
	}
	idB64 := token[:dotIdx]
	sigB64 := token[dotIdx+1:]

	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(idB64))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return "", ErrInvalidToken
	}
	if !hmac.Equal(expectedSig, actualSig) {
		return "", ErrInvalidToken
	}

	idBytes, err := base64.RawURLEncoding.DecodeString(idB64)
	if err != nil {
		return "", ErrInvalidToken
	}
	return string(idBytes), nil
}
