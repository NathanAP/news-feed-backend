-- name: CreateArticleFeed :one
INSERT INTO articles_feeds (id, article_id, feed_id)
VALUES (?, ?, ?)
RETURNING *;

-- name: FindArticleFeedsByArticleAndUser :many
-- Returns all valid articles_feeds records for an article that belong to the requesting
-- user. A junction record is only valid when BOTH related rows are active, so the joins
-- filter inactive feeds AND inactive articles (soft-deleted). is_read state is preserved
-- on the rows themselves; they just become invisible while a related side is inactive.
SELECT af.id, af.article_id, af.feed_id, af.is_read, af.created_at, af.modified_at
FROM articles_feeds af
JOIN feeds f ON f.id = af.feed_id
    AND f.user_id = ?
    AND f.status = 1
    AND f.removed_at IS NULL
JOIN articles a ON a.id = af.article_id
    AND a.status = 1
    AND a.removed_at IS NULL
WHERE af.article_id = ?;

-- name: MarkArticleAsReadForUser :exec
-- Marks is_read = 1 on all unread articles_feeds records for a given article and user.
-- Idempotent: already-read records (is_read = 1) are not touched. Both related rows must
-- be active: the article and the feed.
UPDATE articles_feeds
SET is_read = 1, modified_at = CURRENT_TIMESTAMP
WHERE article_id = ?
  AND is_read = 0
  AND article_id IN (
      SELECT id FROM articles
      WHERE status = 1 AND removed_at IS NULL
  )
  AND feed_id IN (
      SELECT id FROM feeds
      WHERE user_id = ? AND status = 1 AND removed_at IS NULL
  );
