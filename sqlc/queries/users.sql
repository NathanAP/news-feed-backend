-- name: CreateUser :one
INSERT INTO users (id, google_id, email, name, picture)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: FindUserByID :one
SELECT * FROM users
WHERE id = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: FindUserByGoogleID :one
SELECT * FROM users
WHERE google_id = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: UpdateUserLastLogin :exec
UPDATE users
SET last_login_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = TRUE AND removed_at IS NULL;

-- name: SoftDeleteUser :exec
UPDATE users
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND removed_at IS NULL;
