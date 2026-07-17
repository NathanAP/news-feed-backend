-- name: CreateArticleFeed :one
INSERT INTO articles_feeds (id, article_id, feed_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: FindArticleFeedsByArticleAndUser :many
-- Returns all valid articles_feeds records for an article that belong to the requesting
-- user. A junction record is only valid when BOTH related rows are active, so the joins
-- filter inactive feeds AND inactive articles (soft-deleted). is_read state is preserved
-- on the rows themselves; they just become invisible while a related side is inactive.
SELECT af.id, af.article_id, af.feed_id, af.is_read, af.created_at, af.modified_at
FROM articles_feeds af
JOIN feeds f ON f.id = af.feed_id
    AND f.user_id = $1
    AND f.status = TRUE
    AND f.removed_at IS NULL
JOIN articles a ON a.id = af.article_id
    AND a.status = TRUE
    AND a.removed_at IS NULL
WHERE af.article_id = $2;

-- name: ListArticlesByFeedForUser :many
-- Returns the active articles associated with a feed, each with its is_read state for that feed
-- and its source's data (always joined, cheap PK lookup, but only mapped into the response when
-- ?with_sources=true, per conventions.md). The article's source is guaranteed active: soft-deleting
-- a source cascades to soft-delete its articles, so an active article always has an active source.
-- Both the feed and the article sides of the junction must be active, and the feed must belong to
-- the requesting user, so another user's feed yields no rows. The caller checks feed ownership
-- separately to distinguish "feed not found / not yours" (404) from "feed has no articles" (200).
-- Filtered and paginated in SQL. All three filters are optional (NULL = not applied): is_read, and
-- the created_at window, whose bounds are independent and inclusive on both ends.
-- Ordered newest-first, with a.id breaking created_at ties so the ordering is total: without it,
-- articles written in the same CRON batch share a timestamp and LIMIT/OFFSET could repeat or skip
-- one across pages. id is a UUIDv7, so the tie-break stays chronological.
-- CountArticlesByFeedForUser below MUST keep the same joins and filters.
SELECT a.id, a.status, a.title, a.content, a.url_original, a.keywords, a.source_id, a.language_original, a.created_at, a.modified_at, af.is_read,
    s.name AS source_name, s.status AS source_status, s.url AS source_url, s.url_rss AS source_url_rss,
    s.created_at AS source_created_at, s.modified_at AS source_modified_at
FROM articles_feeds af
JOIN feeds f ON f.id = af.feed_id
    AND f.user_id = sqlc.arg(user_id)
    AND f.status = TRUE
    AND f.removed_at IS NULL
JOIN articles a ON a.id = af.article_id
    AND a.status = TRUE
    AND a.removed_at IS NULL
JOIN sources s ON s.id = a.source_id
    AND s.status = TRUE
    AND s.removed_at IS NULL
WHERE af.feed_id = sqlc.arg(feed_id)
  AND (sqlc.narg(is_read)::boolean IS NULL OR af.is_read = sqlc.narg(is_read)::boolean)
  AND (sqlc.narg(period_starting_at)::timestamptz IS NULL OR a.created_at >= sqlc.narg(period_starting_at)::timestamptz)
  AND (sqlc.narg(period_ending_at)::timestamptz IS NULL OR a.created_at <= sqlc.narg(period_ending_at)::timestamptz)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountArticlesByFeedForUser :one
-- Total matching rows for the ListArticlesByFeedForUser page, feeding pagination.total_count.
-- Its joins and filters MUST mirror ListArticlesByFeedForUser above, or the envelope lies about the
-- total. The sources join is kept even though no source column is selected: it is what enforces
-- "an article of an inactive source does not count", matching the list exactly.
SELECT COUNT(*)
FROM articles_feeds af
JOIN feeds f ON f.id = af.feed_id
    AND f.user_id = sqlc.arg(user_id)
    AND f.status = TRUE
    AND f.removed_at IS NULL
JOIN articles a ON a.id = af.article_id
    AND a.status = TRUE
    AND a.removed_at IS NULL
JOIN sources s ON s.id = a.source_id
    AND s.status = TRUE
    AND s.removed_at IS NULL
WHERE af.feed_id = sqlc.arg(feed_id)
  AND (sqlc.narg(is_read)::boolean IS NULL OR af.is_read = sqlc.narg(is_read)::boolean)
  AND (sqlc.narg(period_starting_at)::timestamptz IS NULL OR a.created_at >= sqlc.narg(period_starting_at)::timestamptz)
  AND (sqlc.narg(period_ending_at)::timestamptz IS NULL OR a.created_at <= sqlc.narg(period_ending_at)::timestamptz);

-- name: CountUnreadArticlesByFeedForUser :many
-- Counts the unread articles of every active feed owned by the user, for the
-- check-for-new-articles poll endpoint. A junction record only counts when BOTH sides are active
-- (the feed and the article), matching junction-validity rules, and only unread rows are counted.
-- The inner joins plus the is_read filter mean a feed with no unread articles produces no group and
-- is simply absent from the result (the caller renders it as no news). NOTE: keep this comment ASCII
-- only -- sqlc miscounts multibyte UTF-8 bytes here and truncates the tail of the SQL.
SELECT f.id AS feed_id, COUNT(af.id) AS unread_count
FROM feeds f
JOIN articles_feeds af ON af.feed_id = f.id
    AND af.is_read = FALSE
JOIN articles a ON a.id = af.article_id
    AND a.status = TRUE
    AND a.removed_at IS NULL
WHERE f.user_id = $1
    AND f.status = TRUE
    AND f.removed_at IS NULL
GROUP BY f.id;

-- name: MarkArticleAsReadForUser :exec
-- Marks is_read on all unread articles_feeds records for a given article and user.
-- Idempotent: already-read records are not touched. Both related rows must be active:
-- the article and the feed.
UPDATE articles_feeds
SET is_read = TRUE, modified_at = CURRENT_TIMESTAMP
WHERE article_id = $1
  AND is_read = FALSE
  AND article_id IN (
      SELECT id FROM articles
      WHERE status = TRUE AND removed_at IS NULL
  )
  AND feed_id IN (
      SELECT id FROM feeds
      WHERE user_id = $2 AND status = TRUE AND removed_at IS NULL
  );
