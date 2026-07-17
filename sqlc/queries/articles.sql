-- name: CreateArticle :one
INSERT INTO articles (id, title, content, url_original, keywords, source_id, language_original)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: FindArticleByID :one
SELECT * FROM articles
WHERE id = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: FindArticleByURLOriginal :one
SELECT * FROM articles
WHERE url_original = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: ListArticles :many
-- Lists the active articles for GET /v1/articles, filtered and paginated in SQL.
-- The url filter is optional: a NULL param means "no filter" (sqlc.narg), so one query serves both
-- the filtered and the unfiltered case. strpos(lower(a), lower(b)) > 0 is a literal case-insensitive
-- substring test -- deliberately not ILIKE '%...%', which would let a user's % or _ act as wildcards.
-- Neither can use an index (leading wildcard); pg_trgm is the path if that ever matters.
-- id breaks created_at ties so the ordering is total: without it, rows written in the same CRON
-- batch share a timestamp and LIMIT/OFFSET could repeat or skip one across pages. id is a UUIDv7,
-- so it sorts by creation time and the tie-break stays chronological.
-- CountArticles below MUST keep the same filters.
SELECT * FROM articles
WHERE status = TRUE AND removed_at IS NULL
  AND (sqlc.narg(url)::text IS NULL OR strpos(lower(url_original), lower(sqlc.narg(url)::text)) > 0)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListAllArticles :many
-- Every active article, unfiltered and unpaginated, for the dev seed scripts (which need the whole
-- set to build article/feed associations). See ListAllSources for why this is separate from the
-- paginated ListArticles that serves HTTP.
SELECT * FROM articles
WHERE status = TRUE AND removed_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: CountArticles :one
-- Total matching rows for the ListArticles page, feeding pagination.total_count.
-- Its filters MUST mirror ListArticles above, or the envelope lies about the total.
SELECT COUNT(*) FROM articles
WHERE status = TRUE AND removed_at IS NULL
  AND (sqlc.narg(url)::text IS NULL OR strpos(lower(url_original), lower(sqlc.narg(url)::text)) > 0);

-- name: UpdateArticle :one
UPDATE articles
SET title = $1, content = $2, url_original = $3, keywords = $4, language_original = $5, modified_at = CURRENT_TIMESTAMP
WHERE id = $6 AND status = TRUE AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteArticle :exec
UPDATE articles
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND removed_at IS NULL;

-- name: SoftDeleteArticlesBySourceID :exec
UPDATE articles
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE source_id = $1 AND removed_at IS NULL;
