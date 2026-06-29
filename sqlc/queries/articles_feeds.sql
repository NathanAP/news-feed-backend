-- name: CreateArticleFeed :one
INSERT INTO articles_feeds (id, article_id, feed_id)
VALUES (?, ?, ?)
RETURNING *;

-- name: FindArticleFeedsByArticleAndUser :many
-- Returns all active articles_feeds records for an article that belong to the requesting
-- user. The join with feeds filters out inactive feeds (soft-deleted), ensuring only
-- valid associations are returned. is_read state is preserved on soft-deleted feeds.
SELECT af.id, af.article_id, af.feed_id, af.is_read, af.created_at, af.modified_at
FROM articles_feeds af
JOIN feeds f ON f.id = af.feed_id
    AND f.user_id = ?
    AND f.status = 1
    AND f.removed_at IS NULL
WHERE af.article_id = ?;

-- name: MarkArticleAsReadForUser :exec
-- Marks is_read = 1 on all unread articles_feeds records for a given article and user.
-- Idempotent: already-read records (is_read = 1) are not touched.
UPDATE articles_feeds
SET is_read = 1, modified_at = CURRENT_TIMESTAMP
WHERE article_id = ?
  AND is_read = 0
  AND feed_id IN (
      SELECT id FROM feeds
      WHERE user_id = ? AND status = 1 AND removed_at IS NULL
  );
