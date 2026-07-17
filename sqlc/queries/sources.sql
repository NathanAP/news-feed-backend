-- name: CreateSource :one
INSERT INTO sources (id, name, url, url_rss)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: FindSourceByID :one
SELECT * FROM sources
WHERE id = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: ListSources :many
-- Lists the active sources for GET /v1/sources, filtered and paginated in SQL.
-- url and name are independent optional substring filters (NULL = not applied); passing both ANDs
-- them. See ListArticles for why strpos over ILIKE and why id breaks the created_at tie.
-- CountSources below MUST keep the same filters.
SELECT * FROM sources
WHERE status = TRUE AND removed_at IS NULL
  AND (sqlc.narg(url)::text IS NULL OR strpos(lower(url), lower(sqlc.narg(url)::text)) > 0)
  AND (sqlc.narg(name)::text IS NULL OR strpos(lower(name), lower(sqlc.narg(name)::text)) > 0)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListAllSources :many
-- Every active source, unfiltered and unpaginated, for the internal batch consumers: the CRON's
-- discovery sweep (which must visit ALL sources, not a page of them) and the dev seed scripts.
-- Deliberately separate from ListSources: paging a sweep would silently skip sources, and giving
-- the batch callers a page_size big enough to "fit everything" would be a bug waiting for the
-- source count to grow past it. HTTP clients must use ListSources, which is always paginated.
SELECT * FROM sources
WHERE status = TRUE AND removed_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: CountSources :one
-- Total matching rows for the ListSources page, feeding pagination.total_count.
-- Its filters MUST mirror ListSources above, or the envelope lies about the total.
SELECT COUNT(*) FROM sources
WHERE status = TRUE AND removed_at IS NULL
  AND (sqlc.narg(url)::text IS NULL OR strpos(lower(url), lower(sqlc.narg(url)::text)) > 0)
  AND (sqlc.narg(name)::text IS NULL OR strpos(lower(name), lower(sqlc.narg(name)::text)) > 0);

-- name: UpdateSource :one
UPDATE sources
SET name = $1, url = $2, url_rss = $3, modified_at = CURRENT_TIMESTAMP
WHERE id = $4 AND status = TRUE AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteSource :exec
UPDATE sources
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND removed_at IS NULL;
