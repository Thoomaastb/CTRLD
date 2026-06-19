-- name: CreatePIMSession :one
INSERT INTO pim_sessions (
    id,
    user_id,
    session_id,
    reason,
    requested_duration_min,
    started_at,
    expires_at,
    is_break_glass,
    action_count
) VALUES (
    ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, 0
)
RETURNING *;

-- name: GetActivePIMSessionByUserID :one
SELECT * FROM pim_sessions
WHERE user_id = ?
  AND ended_at IS NULL
  AND expires_at > CURRENT_TIMESTAMP
LIMIT 1;

-- name: GetPIMSessionByID :one
SELECT * FROM pim_sessions
WHERE id = ?
LIMIT 1;

-- name: EndPIMSession :exec
UPDATE pim_sessions
SET ended_at = CURRENT_TIMESTAMP
WHERE id = ? AND ended_at IS NULL;

-- name: IncrementPIMActionCount :exec
UPDATE pim_sessions
SET action_count = action_count + 1
WHERE id = ? AND ended_at IS NULL;

-- name: ExpireOverduePIMSessions :exec
UPDATE pim_sessions
SET ended_at = CURRENT_TIMESTAMP
WHERE ended_at IS NULL AND expires_at <= CURRENT_TIMESTAMP;
