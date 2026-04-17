-- Combined init script for Docker
-- This runs before the numbered migrations

-- Identities
CREATE TABLE IF NOT EXISTS identities (
    kryla_id   TEXT PRIMARY KEY,
    public_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_identities_public_key ON identities (public_key);

-- Signed prekeys
CREATE TABLE IF NOT EXISTS signed_prekeys (
    kryla_id       TEXT PRIMARY KEY REFERENCES identities(kryla_id) ON DELETE CASCADE,
    signed_pre_key TEXT NOT NULL,
    signature      TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One-time prekeys
CREATE TABLE IF NOT EXISTS one_time_prekeys (
    kryla_id   TEXT NOT NULL REFERENCES identities(kryla_id) ON DELETE CASCADE,
    key_id     TEXT NOT NULL,
    public_key TEXT NOT NULL,
    used       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (kryla_id, key_id)
);
CREATE INDEX IF NOT EXISTS idx_otp_available ON one_time_prekeys (kryla_id, used, created_at)
    WHERE used = FALSE;

-- Offline messages
CREATE TABLE IF NOT EXISTS offline_messages (
    id           BIGSERIAL PRIMARY KEY,
    recipient_id TEXT NOT NULL,
    message_id   TEXT NOT NULL UNIQUE,
    sender_id    TEXT NOT NULL,
    encrypted    TEXT NOT NULL,
    header       TEXT NOT NULL,
    stored_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_offline_recipient ON offline_messages (recipient_id, stored_at);
