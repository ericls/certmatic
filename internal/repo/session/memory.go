package session

import (
	"sync"
	"sync/atomic"
	"time"

	pkgsession "github.com/ericls/certmatic/pkg/session"
)

type sessionEntry struct {
	session  *pkgsession.Session
	redeemed atomic.Bool
}

// MemorySessionStore is an in-memory SessionStore implementation.
// It survives Caddy config hot-reloads when kept in the usagePool.
type MemorySessionStore struct {
	sessions sync.Map // map[string]*sessionEntry
}

// Destruct implements caddy.Destructor — stops the background cleanup goroutine.
func (s *MemorySessionStore) Destruct() error {
	return nil
}

// NewMemorySessionStore returns a new in-memory session store and starts a background
// goroutine that periodically evicts expired sessions.
func NewMemorySessionStore() *MemorySessionStore {
	s := &MemorySessionStore{}
	return s
}

// StoreSession persists a newly created session.
func (s *MemorySessionStore) StoreSession(session *pkgsession.Session) error {
	entry := &sessionEntry{session: session}
	s.sessions.Store(session.SessionID, entry)
	return nil
}

// RedeemToken validates the HMAC-signed token and returns the stored session (one-time use).
func (s *MemorySessionStore) RedeemToken(signingKey []byte, token string) (*pkgsession.Session, error) {
	sessionID, err := pkgsession.VerifyTokenGetSessionID(signingKey, token)
	if err != nil {
		return nil, err
	}

	val, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, pkgsession.ErrInvalidToken
	}
	entry := val.(*sessionEntry)

	if time.Now().After(entry.session.ExpiresAt) {
		s.sessions.Delete(sessionID)
		return nil, pkgsession.ErrExpiredToken
	}

	// Swap returns the old value; if it was already true the token was already redeemed.
	if entry.redeemed.Swap(true) {
		return nil, pkgsession.ErrTokenReplayed
	}

	return entry.session, nil
}

// GetSession returns an active session by ID, pruning expired sessions lazily.
func (s *MemorySessionStore) GetSession(sessionID string) (*pkgsession.Session, error) {
	val, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, pkgsession.ErrInvalidToken
	}
	entry := val.(*sessionEntry)
	if time.Now().After(entry.session.ExpiresAt) {
		s.sessions.Delete(sessionID)
		return nil, pkgsession.ErrExpiredToken
	}
	return entry.session, nil
}

// ClearExpired removes all sessions that have passed their expiry time.
func (s *MemorySessionStore) ClearExpired() error {
	now := time.Now()
	s.sessions.Range(func(key, val any) bool {
		if now.After(val.(*sessionEntry).session.ExpiresAt) {
			s.sessions.Delete(key)
		}
		return true
	})
	return nil
}

var _ pkgsession.SessionStore = (*MemorySessionStore)(nil)
