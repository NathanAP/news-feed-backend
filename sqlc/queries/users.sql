-- name: CreateUser :one
INSERT INTO users (id, google_id, email, name, picture)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: FindUserByID :one
SELECT * FROM users
WHERE id = ? AND status = 1 AND removed_at IS NULL
LIMIT 1;

-- name: FindUserByGoogleID :one
SELECT * FROM users
WHERE google_id = ? AND status = 1 AND removed_at IS NULL
LIMIT 1;

-- name: UpdateUserLastLogin :exec
UPDATE users
SET last_login_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = 1 AND removed_at IS NULL;

-- name: SoftDeleteUser :exec
UPDATE users
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND removed_at IS NULL;
