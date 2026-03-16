package portal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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

// Session represents an authenticated portal session scoped to a single hostname.
type Session struct {
	SessionID string
	Hostname  string
	ExpiresAt time.Time
	BackURL   string
	BackText  string
}

// SessionStore manages portal sessions.
type SessionStore interface {
	// RedeemToken validates an HMAC-signed token (one-time use) and stores the resulting session.
	RedeemToken(signingKey []byte, token string) (*Session, error)
	// GetSession looks up an active session by session ID.
	GetSession(sessionID string) (*Session, error)
}

type tokenPayload struct {
	Hostname  string    `json:"hostname"`
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
	BackURL   string    `json:"back_url,omitempty"`
	BackText  string    `json:"back_text,omitempty"`
}

// CreateToken generates a new HMAC-signed portal token for the given hostname.
// Returns the token string and its expiry time.
func CreateToken(signingKey []byte, hostname string, ttl time.Duration, backURL, backText string) (string, time.Time, error) {
	sessionID, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate session id: %w", err)
	}
	payload := tokenPayload{
		Hostname:  hostname,
		SessionID: sessionID.String(),
		ExpiresAt: time.Now().UTC().Add(ttl),
		BackURL:   backURL,
		BackText:  backText,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal payload: %w", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(payloadB64))
	sig := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	token := payloadB64 + "." + sigB64
	return token, payload.ExpiresAt, nil
}

func verifyToken(signingKey []byte, token string) (*tokenPayload, error) {
	// Find the last dot separating payload from signature.
	dotIdx := -1
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			dotIdx = i
			break
		}
	}
	if dotIdx < 0 {
		return nil, ErrInvalidToken
	}
	payloadB64 := token[:dotIdx]
	sigB64 := token[dotIdx+1:]

	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(payloadB64))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !hmac.Equal(expectedSig, actualSig) {
		return nil, ErrInvalidToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, ErrInvalidToken
	}
	var payload tokenPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().After(payload.ExpiresAt) {
		return nil, ErrExpiredToken
	}

	return &payload, nil
}
