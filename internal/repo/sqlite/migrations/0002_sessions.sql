CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    redeemed INTEGER NOT NULL DEFAULT 0,
    back_url TEXT NOT NULL DEFAULT '',
    back_text TEXT NOT NULL DEFAULT '',
    ownership_verification_mode TEXT NOT NULL DEFAULT '',
    verify_ownership_url TEXT NOT NULL DEFAULT '',
    verify_ownership_text TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
