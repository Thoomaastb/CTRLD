-- +goose Up
-- +goose StatementBegin

-- ─── pim_sessions ─────────────────────────────────────────────────────────────
CREATE TABLE pim_sessions (
    id                     TEXT PRIMARY KEY,                 -- UUID v4
    user_id                TEXT NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    session_id             TEXT NOT NULL REFERENCES sessions (id) ON DELETE RESTRICT,
    reason                 TEXT NOT NULL,                    -- min. 10 Zeichen (App-Ebene)
    requested_duration_min INTEGER NOT NULL,
    started_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at             DATETIME NOT NULL,
    ended_at               DATETIME,                         -- NULL = aktiv
    is_break_glass         INTEGER NOT NULL DEFAULT 0
                           CHECK (is_break_glass IN (0, 1)),
    action_count           INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_pim_user_id    ON pim_sessions (user_id);
CREATE INDEX idx_pim_expires_at ON pim_sessions (expires_at);
CREATE INDEX idx_pim_ended_at   ON pim_sessions (ended_at);

-- ─── audit_log (append-only) ──────────────────────────────────────────────────
CREATE TABLE audit_log (
    id             TEXT PRIMARY KEY,                         -- UUID v4
    user_id        TEXT REFERENCES users (id) ON DELETE SET NULL,
    session_id     TEXT REFERENCES sessions (id) ON DELETE SET NULL,
    pim_session_id TEXT REFERENCES pim_sessions (id) ON DELETE SET NULL,
    action_type    TEXT NOT NULL,                            -- z.B. "service.restart"
    resource       TEXT,                                     -- z.B. "nginx.service"
    result         TEXT NOT NULL
                   CHECK (result IN ('success', 'failure')),
    ip_address     TEXT,
    metadata       TEXT,                                     -- JSON
    severity       TEXT NOT NULL
                   CHECK (severity IN ('info', 'warning', 'critical')),
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_user_id    ON audit_log (user_id);
CREATE INDEX idx_audit_created_at ON audit_log (created_at);
CREATE INDEX idx_audit_severity   ON audit_log (severity);
CREATE INDEX idx_audit_action     ON audit_log (action_type);

-- Append-Only Schutz via DB-Trigger
-- Kein UPDATE erlaubt
CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only: UPDATE not permitted');
END;

-- Kein DELETE erlaubt
CREATE TRIGGER audit_log_no_delete
    BEFORE DELETE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only: DELETE not permitted');
END;

-- ─── config ───────────────────────────────────────────────────────────────────
CREATE TABLE config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,                                -- JSON oder String
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by TEXT REFERENCES users (id) ON DELETE SET NULL
);

-- ─── alerts ───────────────────────────────────────────────────────────────────
CREATE TABLE alerts (
    id            TEXT PRIMARY KEY,                          -- UUID v4
    type          TEXT NOT NULL
                  CHECK (type IN ('cpu', 'ram', 'disk')),
    threshold     REAL NOT NULL,
    current_value REAL NOT NULL,
    severity      TEXT NOT NULL
                  CHECK (severity IN ('warning', 'critical')),
    triggered_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at   DATETIME,                                  -- NULL = aktiv
    notified      INTEGER NOT NULL DEFAULT 0
                  CHECK (notified IN (0, 1))
);

CREATE INDEX idx_alerts_resolved_at ON alerts (resolved_at);
CREATE INDEX idx_alerts_severity    ON alerts (severity);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS audit_log_no_delete;
DROP TRIGGER IF EXISTS audit_log_no_update;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS config;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS pim_sessions;
-- +goose StatementEnd
