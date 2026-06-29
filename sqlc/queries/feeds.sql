-- name: CreateFeed :one
INSERT INTO feeds (id, name, keywords, user_id)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: FindFeedByIDAndUser :one
SELECT * FROM feeds
WHERE id = ? AND user_id = ? AND status = 1 AND removed_at IS NULL
LIMIT 1;

-- name: ListFeedsByUser :many
SELECT * FROM feeds
WHERE user_id = ? AND status = 1 AND removed_at IS NULL
ORDER BY created_at DESC;

-- name: CountActiveFeedsByUser :one
SELECT COUNT(*) FROM feeds
WHERE user_id = ? AND status = 1 AND removed_at IS NULL;

-- name: UpdateFeedByIDAndUser :one
UPDATE feeds
SET name = ?, keywords = ?, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND user_id = ? AND status = 1 AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteFeedByIDAndUser :exec
UPDATE feeds
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND user_id = ? AND removed_at IS NULL;

-- name: SoftDeleteFeedsByUser :exec
UPDATE feeds
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE user_id = ? AND removed_at IS NULL;
