CREATE TABLE IF NOT EXISTS identities (
    kryla_id   TEXT PRIMARY KEY,
    public_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_identities_public_key ON identities (public_key);
