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

-- name: ListArticlesByFeedForUser :many
-- Returns the active articles associated with a feed, each with its is_read state for that feed
-- and its source's data (always joined, cheap PK lookup, but only mapped into the response when
-- ?with_sources=true, per conventions.md). The article's source is guaranteed active: soft-deleting
-- a source cascades to soft-delete its articles, so an active article always has an active source.
-- Both the feed and the article sides of the junction must be active, and the feed must belong to
-- the requesting user, so another user's feed yields no rows. The caller checks feed ownership
-- separately to distinguish "feed not found / not yours" (404) from "feed has no articles" (200).
-- Ordered newest-first; is_read / date filtering and pagination are applied by the caller.
SELECT a.id, a.status, a.title, a.content, a.url_original, a.keywords, a.source_id, a.language_original, a.created_at, a.modified_at, af.is_read,
    s.name AS source_name, s.status AS source_status, s.url AS source_url, s.url_rss AS source_url_rss,
    s.created_at AS source_created_at, s.modified_at AS source_modified_at
FROM articles_feeds af
JOIN feeds f ON f.id = af.feed_id
    AND f.user_id = ?
    AND f.status = 1
    AND f.removed_at IS NULL
JOIN articles a ON a.id = af.article_id
    AND a.status = 1
    AND a.removed_at IS NULL
JOIN sources s ON s.id = a.source_id
    AND s.status = 1
    AND s.removed_at IS NULL
WHERE af.feed_id = ?
ORDER BY a.created_at DESC;

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
