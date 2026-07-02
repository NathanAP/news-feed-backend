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

-- name: FindCandidateFeedsByKeywords :many
-- Judgement layer 1 (keyword overlap): returns every active feed (of any user) that shares at
-- least one keyword with the article. Both sides are stored as JSON arrays of lowercase strings,
-- so json_each expands each into rows and the join matches on exact keyword equality. DISTINCT
-- collapses a feed that overlaps on several keywords into a single row. The parameter is the
-- article's keywords as a JSON array TEXT.
SELECT DISTINCT f.id, f.status, f.name, f.keywords, f.user_id, f.created_at, f.modified_at, f.removed_at
FROM feeds f
JOIN json_each(f.keywords) fk
JOIN json_each(?) ak ON ak.value = fk.value
WHERE f.status = 1 AND f.removed_at IS NULL;

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
