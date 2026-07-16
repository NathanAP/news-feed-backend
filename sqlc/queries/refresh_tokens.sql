-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (id, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: FindRefreshTokenByID :one
SELECT * FROM refresh_tokens
WHERE id = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: ExtendRefreshToken :exec
UPDATE refresh_tokens
SET expires_at = $1, modified_at = CURRENT_TIMESTAMP
WHERE id = $2 AND status = TRUE AND removed_at IS NULL;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND removed_at IS NULL;

-- name: RevokeAllRefreshTokensByUserID :exec
UPDATE refresh_tokens
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND removed_at IS NULL;
