-- Offline queue is primarily Redis-backed, but we keep a PostgreSQL table
-- as a durable fallback for messages that must survive Redis restarts.
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
