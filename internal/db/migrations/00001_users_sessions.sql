-- +goose Up
-- +goose StatementBegin

-- ─── users ────────────────────────────────────────────────────────────────────
CREATE TABLE users (
    id            TEXT PRIMARY KEY,                          -- UUID v4
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,                             -- Argon2id
    role          TEXT NOT NULL DEFAULT 'viewer'
                  CHECK (role IN ('admin', 'viewer')),
    backup_codes  TEXT,                                      -- JSON-Array, gehasht
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at DATETIME,
    is_active     INTEGER NOT NULL DEFAULT 1                 -- SQLite BOOLEAN
                  CHECK (is_active IN (0, 1))
);

CREATE INDEX idx_users_email    ON users (email);
CREATE INDEX idx_users_is_active ON users (is_active);

-- ─── mfa_credentials ──────────────────────────────────────────────────────────
CREATE TABLE mfa_credentials (
    id              TEXT PRIMARY KEY,                        -- UUID v4
    user_id         TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type            TEXT NOT NULL
                    CHECK (type IN ('totp', 'passkey', 'fido2')),
    name            TEXT NOT NULL,                           -- z.B. "YubiKey 5"
    credential_data TEXT NOT NULL,                           -- verschlüsselt
    sign_count      INTEGER NOT NULL DEFAULT 0,              -- WebAuthn Replay-Schutz
    last_used_at    DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_active       INTEGER NOT NULL DEFAULT 1
                    CHECK (is_active IN (0, 1))
);

CREATE INDEX idx_mfa_user_id   ON mfa_credentials (user_id);
CREATE INDEX idx_mfa_is_active ON mfa_credentials (user_id, is_active);

-- ─── sessions ─────────────────────────────────────────────────────────────────
CREATE TABLE sessions (
    id                  TEXT PRIMARY KEY,                    -- UUID v4
    user_id             TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    access_token_hash   TEXT NOT NULL,
    refresh_token_hash  TEXT NOT NULL,
    ip_address          TEXT NOT NULL,
    user_agent          TEXT,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at          DATETIME NOT NULL,
    revoked_at          DATETIME                             -- NULL = aktiv
);

CREATE INDEX idx_sessions_user_id           ON sessions (user_id);
CREATE INDEX idx_sessions_access_token_hash ON sessions (access_token_hash);
CREATE INDEX idx_sessions_expires_at        ON sessions (expires_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS mfa_credentials;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
