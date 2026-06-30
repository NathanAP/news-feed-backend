-- name: GetSystem :one
SELECT * FROM system
LIMIT 1;

-- name: UpdateSystemAppStatus :one
UPDATE system
SET app_status = ?, modified_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpdateSystemLastArticleDiscovery :one
UPDATE system
SET last_article_discovery_at = ?, modified_at = CURRENT_TIMESTAMP
RETURNING *;
