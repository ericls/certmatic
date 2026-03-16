package portal

import (
	"sync"
	"time"
)

// MemorySessionStore is an in-memory SessionStore implementation.
// It survives Caddy config hot-reloads when kept in the usagePool.
type MemorySessionStore struct {
	sessions sync.Map
}

// Destruct implements caddy.Destructor (no-op for in-memory store).
func (s *MemorySessionStore) Destruct() error {
	return nil
}

// NewMemorySessionStore returns a new in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{}
}

// RedeemToken validates the HMAC-signed token and stores the session (one-time use).
func (s *MemorySessionStore) RedeemToken(signingKey []byte, token string) (*Session, error) {
	payload, err := verifyToken(signingKey, token)
	if err != nil {
		return nil, err
	}

	session := &Session{
		SessionID: payload.SessionID,
		Hostname:  payload.Hostname,
		ExpiresAt: payload.ExpiresAt,
		BackURL:   payload.BackURL,
		BackText:  payload.BackText,
	}

	// LoadOrStore ensures one-time use: if session_id was already redeemed, reject.
	_, loaded := s.sessions.LoadOrStore(payload.SessionID, session)
	if loaded {
		return nil, ErrTokenReplayed
	}

	return session, nil
}

// GetSession returns an active session by ID, pruning expired sessions lazily.
func (s *MemorySessionStore) GetSession(sessionID string) (*Session, error) {
	val, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, ErrInvalidToken
	}
	session := val.(*Session)
	if time.Now().After(session.ExpiresAt) {
		s.sessions.Delete(sessionID)
		return nil, ErrExpiredToken
	}
	return session, nil
}

var _ SessionStore = (*MemorySessionStore)(nil)
