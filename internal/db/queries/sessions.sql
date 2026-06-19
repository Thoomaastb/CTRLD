-- name: CreateSession :one
INSERT INTO sessions (
    id,
    user_id,
    access_token_hash,
    refresh_token_hash,
    ip_address,
    user_agent,
    created_at,
    expires_at
) VALUES (
    ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?
)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = ? AND revoked_at IS NULL
LIMIT 1;

-- name: GetSessionByAccessTokenHash :one
SELECT * FROM sessions
WHERE access_token_hash = ? AND revoked_at IS NULL
LIMIT 1;

-- name: GetSessionByRefreshTokenHash :one
SELECT * FROM sessions
WHERE refresh_token_hash = ? AND revoked_at IS NULL
LIMIT 1;

-- name: ListActiveSessionsByUserID :many
SELECT * FROM sessions
WHERE user_id = ? AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP
ORDER BY created_at DESC;

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: RevokeAllUserSessions :exec
UPDATE sessions
SET revoked_at = CURRENT_TIMESTAMP
WHERE user_id = ? AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < CURRENT_TIMESTAMP AND revoked_at IS NOT NULL;
