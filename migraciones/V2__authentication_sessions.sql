BEGIN;

-- No token, password or refresh credential is stored here. A Firebase context
-- may precede BPM registration, so user_id intentionally has no profile FK.
CREATE TABLE authentication_sessions (
    user_id VARCHAR(128) NOT NULL,
    authenticated_at TIMESTAMPTZ NOT NULL,
    last_activity_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (user_id, authenticated_at)
);

CREATE INDEX authentication_sessions_authenticated_at_idx ON authentication_sessions(authenticated_at);

COMMIT;
