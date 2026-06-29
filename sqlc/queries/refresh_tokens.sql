-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (id, user_id, expires_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: FindRefreshTokenByID :one
SELECT * FROM refresh_tokens
WHERE id = ? AND status = 1 AND removed_at IS NULL
LIMIT 1;

-- name: ExtendRefreshToken :exec
UPDATE refresh_tokens
SET expires_at = ?, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = 1 AND removed_at IS NULL;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND removed_at IS NULL;

-- name: RevokeAllRefreshTokensByUserID :exec
UPDATE refresh_tokens
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE user_id = ? AND removed_at IS NULL;
