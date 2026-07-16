-- name: CreateSource :one
INSERT INTO sources (id, name, url, url_rss)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: FindSourceByID :one
SELECT * FROM sources
WHERE id = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: ListSources :many
SELECT * FROM sources
WHERE status = TRUE AND removed_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateSource :one
UPDATE sources
SET name = $1, url = $2, url_rss = $3, modified_at = CURRENT_TIMESTAMP
WHERE id = $4 AND status = TRUE AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteSource :exec
UPDATE sources
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND removed_at IS NULL;
