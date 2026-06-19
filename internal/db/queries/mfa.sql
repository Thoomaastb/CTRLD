-- name: CreateMFACredential :one
INSERT INTO mfa_credentials (
    id,
    user_id,
    type,
    name,
    credential_data,
    sign_count,
    created_at,
    is_active
) VALUES (
    ?, ?, ?, ?, ?, 0, CURRENT_TIMESTAMP, 1
)
RETURNING *;

-- name: GetMFACredentialByID :one
SELECT * FROM mfa_credentials
WHERE id = ? AND is_active = 1
LIMIT 1;

-- name: ListMFACredentialsByUserID :many
SELECT * FROM mfa_credentials
WHERE user_id = ? AND is_active = 1
ORDER BY created_at ASC;

-- name: UpdateMFASignCount :exec
UPDATE mfa_credentials
SET sign_count    = ?,
    last_used_at  = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeactivateMFACredential :exec
UPDATE mfa_credentials
SET is_active = 0
WHERE id = ? AND user_id = ?;
