-- +goose Up
-- Refresh tokens back a real logout: the access token is short-lived, and
-- revoking the refresh token stops new ones being minted.
--
-- token_hash is the primary key and holds a SHA-256 hex digest, never the token
-- itself, so a leaked dump yields nothing that can be presented.
CREATE TABLE refresh_tokens (
    token_hash TEXT        PRIMARY KEY,
    username   TEXT        NOT NULL,
    role       TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Revoking every token a user holds, and sweeping expired rows, are the two
-- queries that are not by primary key.
CREATE INDEX idx_refresh_tokens_username ON refresh_tokens (username);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens (expires_at);

-- +goose Down
DROP TABLE refresh_tokens;
