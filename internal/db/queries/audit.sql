-- name: CreateAuditEntry :one
INSERT INTO audit_log (
    id,
    user_id,
    session_id,
    pim_session_id,
    action_type,
    resource,
    result,
    ip_address,
    metadata,
    severity,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: ListAuditEntries :many
SELECT * FROM audit_log
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListAuditEntriesByUserID :many
SELECT * FROM audit_log
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListAuditEntriesBySeverity :many
SELECT * FROM audit_log
WHERE severity = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountAuditEntries :one
SELECT COUNT(*) FROM audit_log;
