CREATE TABLE IF NOT EXISTS signed_prekeys (
    kryla_id       TEXT PRIMARY KEY REFERENCES identities(kryla_id) ON DELETE CASCADE,
    signed_pre_key TEXT NOT NULL,
    signature      TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
