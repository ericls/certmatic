package portal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	pkgsession "github.com/ericls/certmatic/pkg/session"
	"github.com/google/uuid"
)

// CreateToken stores a new session and returns an HMAC-signed token containing only the session ID.
func CreateToken(store pkgsession.SessionStore, signingKey []byte, hostname string, ttl time.Duration,
	backURL, backText string, ownershipMode pkgsession.OwnershipVerificationMode,
	verifyOwnershipURL, verifyOwnershipText string,
) (string, time.Time, error) {
	sessionID, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate session id: %w", err)
	}
	expiresAt := time.Now().UTC().Add(ttl)
	sess := &pkgsession.Session{
		SessionID:                 sessionID.String(),
		Hostname:                  hostname,
		ExpiresAt:                 expiresAt,
		BackURL:                   backURL,
		BackText:                  backText,
		OwnershipVerificationMode: ownershipMode,
		VerifyOwnershipURL:        verifyOwnershipURL,
		VerifyOwnershipText:       verifyOwnershipText,
	}
	if err := store.StoreSession(sess); err != nil {
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
