package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

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

// VerifyTokenGetSessionID verifies the HMAC signature and returns the session ID.
func VerifyTokenGetSessionID(signingKey []byte, token string) (string, error) {
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
