-- name: CreateArticle :one
INSERT INTO articles (id, title, content, url_original, keywords, source_id)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: FindArticleByID :one
SELECT * FROM articles
WHERE id = ? AND status = 1 AND removed_at IS NULL
LIMIT 1;

-- name: ListArticles :many
SELECT * FROM articles
WHERE status = 1 AND removed_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateArticle :one
UPDATE articles
SET title = ?, content = ?, url_original = ?, keywords = ?, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = 1 AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteArticle :exec
UPDATE articles
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = ? AND removed_at IS NULL;

-- name: SoftDeleteArticlesBySourceID :exec
UPDATE articles
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE source_id = ? AND removed_at IS NULL;
