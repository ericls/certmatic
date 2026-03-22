CREATE TABLE IF NOT EXISTS domains (
    hostname TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    ownership_verified INTEGER NOT NULL DEFAULT 0,
    verification_token TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
