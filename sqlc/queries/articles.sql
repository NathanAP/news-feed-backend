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
SELECT * FROM articles
WHERE status = TRUE AND removed_at IS NULL
ORDER BY created_at DESC;

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
