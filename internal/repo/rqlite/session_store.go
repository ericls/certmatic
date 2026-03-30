package rqlite

import (
	"fmt"
	"time"

	gorqlite "github.com/rqlite/gorqlite"

	pkgsession "github.com/ericls/certmatic/pkg/session"
)

// SessionStore is an rqlite-backed session.SessionStore.
type SessionStore struct {
	httpAddr string
	sc       *sharedConn
}

// NewSessionStore opens (or reuses) the rqlite connection at httpAddr and returns a SessionStore.
func NewSessionStore(httpAddr string) (*SessionStore, error) {
	sc, err := acquireConn(httpAddr)
	if err != nil {
		return nil, err
	}
	return &SessionStore{httpAddr: httpAddr, sc: sc}, nil
}

// Destruct implements caddy.Destructor — releases the shared connection.
func (s *SessionStore) Destruct() error {
	releaseConn(s.httpAddr)
	return nil
}

// StoreSession implements session.SessionStore.
func (s *SessionStore) StoreSession(session *pkgsession.Session) error {
	s.sc.mu.Lock()
	defer s.sc.mu.Unlock()

	wr, err := s.sc.conn.WriteOneParameterized(gorqlite.ParameterizedStatement{
		Query: `INSERT INTO sessions
			   (session_id, hostname, expires_at, redeemed, back_url, back_text,
			    ownership_verification_mode, verify_ownership_url, verify_ownership_text)
			 VALUES (?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		Arguments: []interface{}{
			session.SessionID,
			session.Hostname,
			session.ExpiresAt.UTC().Format(time.RFC3339),
			session.BackURL,
			session.BackText,
			string(session.OwnershipVerificationMode),
			session.VerifyOwnershipURL,
			session.VerifyOwnershipText,
		},
	})
	if err != nil {
		return fmt.Errorf("store session %q: %w", session.SessionID, err)
	}
	if wr.Err != nil {
		return fmt.Errorf("store session %q: %w", session.SessionID, wr.Err)
	}
	return nil
}

// RedeemToken implements session.SessionStore.
func (s *SessionStore) RedeemToken(signingKey []byte, token string) (*pkgsession.Session, error) {
	sess, err := s.getSessionByToken(signingKey, token)
	if err != nil {
		return nil, err
	}

	s.sc.mu.Lock()
	defer s.sc.mu.Unlock()

	// Atomic one-time redemption: only succeeds if not yet redeemed.
	wr, err := s.sc.conn.WriteOneParameterized(gorqlite.ParameterizedStatement{
		Query:     `UPDATE sessions SET redeemed = 1 WHERE session_id = ? AND redeemed = 0`,
		Arguments: []interface{}{sess.SessionID},
	})
	if err != nil {
		return nil, fmt.Errorf("redeem session %q: %w", sess.SessionID, err)
	}
	if wr.Err != nil {
		return nil, fmt.Errorf("redeem session %q: %w", sess.SessionID, wr.Err)
	}
	if wr.RowsAffected == 0 {
		return nil, pkgsession.ErrTokenReplayed
	}

	return sess, nil
}

// GetSession implements session.SessionStore.
func (s *SessionStore) GetSession(sessionID string) (*pkgsession.Session, error) {
	sess, err := s.loadSession(sessionID)
	if err != nil {
		return nil, err
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, pkgsession.ErrExpiredToken
	}
	return sess, nil
}

// ClearExpired implements session.SessionStore.
func (s *SessionStore) ClearExpired() error {
	s.sc.mu.Lock()
	defer s.sc.mu.Unlock()

	wr, err := s.sc.conn.WriteOneParameterized(gorqlite.ParameterizedStatement{
		Query:     `DELETE FROM sessions WHERE expires_at < ?`,
		Arguments: []interface{}{time.Now().UTC().Format(time.RFC3339)},
	})
	if err != nil {
		return err
	}
	if wr.Err != nil {
		return wr.Err
	}
	return nil
}

func (s *SessionStore) getSessionByToken(signingKey []byte, token string) (*pkgsession.Session, error) {
	sessionID, err := pkgsession.VerifyTokenGetSessionID(signingKey, token)
	if err != nil {
		return nil, err
	}
	sess, err := s.loadSession(sessionID)
	if err != nil {
		return nil, err
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, pkgsession.ErrExpiredToken
	}
	return sess, nil
}

func (s *SessionStore) loadSession(sessionID string) (*pkgsession.Session, error) {
	s.sc.mu.Lock()
	defer s.sc.mu.Unlock()

	qr, err := s.sc.conn.QueryOneParameterized(gorqlite.ParameterizedStatement{
		Query: `SELECT session_id, hostname, expires_at, back_url, back_text,
			        ownership_verification_mode, verify_ownership_url, verify_ownership_text
			 FROM sessions WHERE session_id = ?`,
		Arguments: []interface{}{sessionID},
	})
	if err != nil {
		return nil, fmt.Errorf("load session %q: %w", sessionID, err)
	}
	if qr.Err != nil {
		return nil, fmt.Errorf("load session %q: %w", sessionID, qr.Err)
	}

	if !qr.Next() {
		return nil, pkgsession.ErrInvalidToken
	}

	var sess pkgsession.Session
	var expiresAtStr string
	var ownershipMode string
	if err := qr.Scan(
		&sess.SessionID, &sess.Hostname, &expiresAtStr,
		&sess.BackURL, &sess.BackText,
		&ownershipMode, &sess.VerifyOwnershipURL, &sess.VerifyOwnershipText,
	); err != nil {
		return nil, fmt.Errorf("load session %q: %w", sessionID, err)
	}

	sess.ExpiresAt, err = time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse expires_at for session %q: %w", sessionID, err)
	}
	sess.OwnershipVerificationMode = pkgsession.OwnershipVerificationMode(ownershipMode)

	return &sess, nil
}

var _ pkgsession.SessionStore = (*SessionStore)(nil)
