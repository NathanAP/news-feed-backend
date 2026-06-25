-- name: CreateSource :one
INSERT INTO sources (id, url, url_rss)
VALUES (?, ?, ?)
RETURNING *;

-- name: FindSourceByID :one
SELECT * FROM sources
WHERE id = ? AND removed_at IS NULL
LIMIT 1;

-- name: ListSources :many
SELECT * FROM sources
WHERE removed_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateSource :one
UPDATE sources
SET url = ?, url_rss = ?, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteSource :exec
UPDATE sources
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND removed_at IS NULL;
