-- name: CreateUser :one
INSERT INTO users (
    id,
    email,
    password_hash,
    role,
    backup_codes,
    created_at,
    is_active
) VALUES (
    ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, 1
)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = ? AND is_active = 1
LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = ? AND is_active = 1
LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC;

-- name: UpdateUserRole :one
UPDATE users
SET role = ?
WHERE id = ? AND is_active = 1
RETURNING *;

-- name: UpdateLastLogin :exec
UPDATE users
SET last_login_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeactivateUser :exec
UPDATE users
SET is_active = 0
WHERE id = ?;
