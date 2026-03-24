CREATE TABLE IF NOT EXISTS api_sessions (
    token text PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_api_sessions_expires_at
    ON api_sessions (expires_at);
