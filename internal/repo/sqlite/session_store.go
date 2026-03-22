package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	pkgsession "github.com/ericls/certmatic/pkg/session"
)

// SessionStore is a SQLite-backed session.SessionStore.
// Multiple SessionStore instances pointing to the same file share a single *sql.DB.
type SessionStore struct {
	filePath string
	db       *sql.DB
}

// NewSessionStore opens (or reuses) the SQLite database at filePath and returns a SessionStore.
func NewSessionStore(filePath string) (*SessionStore, error) {
	db, err := acquireDB(filePath)
	if err != nil {
		return nil, err
	}
	return &SessionStore{filePath: filePath, db: db}, nil
}

// Destruct implements caddy.Destructor — releases the shared DB connection.
func (s *SessionStore) Destruct() error {
	releaseDB(s.filePath)
	return nil
}

// StoreSession implements session.SessionStore.
func (s *SessionStore) StoreSession(session *pkgsession.Session) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO sessions
		   (session_id, hostname, expires_at, redeemed, back_url, back_text,
		    ownership_verification_mode, verify_ownership_url, verify_ownership_text)
		 VALUES (?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		session.SessionID,
		session.Hostname,
		session.ExpiresAt.UTC().Format(time.RFC3339),
		session.BackURL,
		session.BackText,
		string(session.OwnershipVerificationMode),
		session.VerifyOwnershipURL,
		session.VerifyOwnershipText,
	)
	if err != nil {
		return fmt.Errorf("store session %q: %w", session.SessionID, err)
	}
	return nil
}

// RedeemToken implements session.SessionStore.
func (s *SessionStore) RedeemToken(signingKey []byte, token string) (*pkgsession.Session, error) {
	sess, err := s.getSessionByToken(signingKey, token)
	if err != nil {
		return nil, err
	}

	// Atomic one-time redemption: only succeeds if not yet redeemed.
	res, err := s.db.ExecContext(context.Background(),
		`UPDATE sessions SET redeemed = 1 WHERE session_id = ? AND redeemed = 0`,
		sess.SessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("redeem session %q: %w", sess.SessionID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
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
	_, err := s.db.ExecContext(context.Background(),
		`DELETE FROM sessions WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
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
	row := s.db.QueryRowContext(context.Background(),
		`SELECT session_id, hostname, expires_at, back_url, back_text,
		        ownership_verification_mode, verify_ownership_url, verify_ownership_text
		 FROM sessions WHERE session_id = ?`,
		sessionID,
	)

	var sess pkgsession.Session
	var expiresAtStr string
	var ownershipMode string
	err := row.Scan(
		&sess.SessionID, &sess.Hostname, &expiresAtStr,
		&sess.BackURL, &sess.BackText,
		&ownershipMode, &sess.VerifyOwnershipURL, &sess.VerifyOwnershipText,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, pkgsession.ErrInvalidToken
	}
	if err != nil {
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
