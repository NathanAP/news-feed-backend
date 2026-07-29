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

-- SetUserAdmin is deliberately separate from CreateUser instead of an `admin` parameter on it: the
-- Google login flow is the only caller of CreateUser, and keeping the column out of that INSERT makes
-- it structurally impossible for a login to mint an administrator. Promotion is an explicit, separate
-- act. Today only the development seed calls this; a future admin-management endpoint would too.
-- name: SetUserAdmin :one
UPDATE users
SET admin = $2, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = TRUE AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteUser :exec
UPDATE users
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND removed_at IS NULL;
