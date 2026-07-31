-- name: CreateFeed :one
INSERT INTO feeds (id, name, keywords, user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: FindFeedByIDAndUser :one
SELECT * FROM feeds
WHERE id = $1 AND user_id = $2 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: ListFeedsByUser :many
-- Lists the requesting user's active feeds for GET /v1/feeds, filtered and paginated in SQL.
-- The name filter is optional (NULL = not applied). See ListArticles for why strpos over ILIKE and
-- why id breaks the created_at tie. CountFeedsByUser below MUST keep the same filters.
SELECT * FROM feeds
WHERE user_id = sqlc.arg(user_id) AND status = TRUE AND removed_at IS NULL
  AND (sqlc.narg(name)::text IS NULL OR strpos(lower(name), lower(sqlc.narg(name)::text)) > 0)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListAllFeedsByUser :many
-- Every active feed of a user, unfiltered and unpaginated, for the dev seed scripts. See
-- ListAllSources for why this is separate from the paginated ListFeedsByUser that serves HTTP.
SELECT * FROM feeds
WHERE user_id = $1 AND status = TRUE AND removed_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: CountFeedsByUser :one
-- Total matching rows for the ListFeedsByUser page, feeding pagination.total_count.
-- Its filters MUST mirror ListFeedsByUser above, or the envelope lies about the total.
-- NOTE: this is NOT the query that guards the 5-active-feeds-per-user rule -- that one is
-- CountActiveFeedsByUser below, which must never see a name filter.
SELECT COUNT(*) FROM feeds
WHERE user_id = sqlc.arg(user_id) AND status = TRUE AND removed_at IS NULL
  AND (sqlc.narg(name)::text IS NULL OR strpos(lower(name), lower(sqlc.narg(name)::text)) > 0);

-- name: CountActiveFeedsByUser :one
-- Counts every active feed of a user, enforcing the max-active-feeds business rule at creation
-- time. Intentionally unfiltered: the limit is about how many feeds exist, not about a search.
SELECT COUNT(*) FROM feeds
WHERE user_id = $1 AND status = TRUE AND removed_at IS NULL;

-- name: FindCandidateFeedsByKeywords :many
-- Judgement layer 1 (keyword overlap): returns every active feed (of any user) that shares at
-- least one keyword with the article, along with overlap_count (how many distinct keywords matched).
-- Restricted to feeds owned by an ACTIVE user (0.43): the join to users drops feeds whose owner has
-- not been seen within inactive_days, so the CRON stops routing news (and paying for AI) to abandoned
-- accounts. inactive_days = -1 disables the window (every user counts as active); a positive N means
-- last_active_at must be within N days. The owner must also be active (status/removed_at) like every
-- other related lookup. Judgement is not retroactive, so an account that comes back has permanently
-- missed the articles discovered while it was inactive - this is the accepted trade-off.
-- Both sides are JSONB arrays of lowercase strings, so jsonb_array_elements_text expands each into
-- rows and the join matches on exact keyword equality. The LATERAL is what lets the expansion of
-- f.keywords reference the feed row being scanned; the article's side does not depend on the row, so
-- it is a plain join. GROUP BY collapses a feed to one row and COUNT gives its overlap (grouping by
-- f.id alone is valid because it is the primary key, so the other f.* columns are functionally
-- dependent on it). The overlap feeds the triage (auto-associate / discard / send-to-AI) in layer 2.
-- The parameter is the article's keywords as a JSON array.
--
-- The `?|` predicate ("does f.keywords contain ANY of these keys") is what makes idx_feeds_keywords
-- (GIN) usable: an index is matched by OPERATOR, and expanding a column through
-- jsonb_array_elements_text in a LATERAL is a function call on every row, which no index can serve.
-- Without it the planner reads every active feed of every user and expands its keywords just to
-- throw almost all of them away (measured on 60k feeds: 428ms/10.2k buffers versus 22ms/836 with it,
-- where it becomes a Bitmap Index Scan on idx_feeds_keywords).
--
-- It is logically redundant with the join below (a feed with zero shared keywords produces no row
-- either way), so it cannot change the result set - it only lets the planner discard non-candidates
-- before the expensive expansion. It must be kept in sync with the join's matching rule: both sides
-- compare the same lowercase text keys.
SELECT f.id, f.status, f.name, f.keywords, f.user_id, f.created_at, f.modified_at, f.removed_at,
       COUNT(DISTINCT fk.value) AS overlap_count
FROM feeds f
JOIN users u ON u.id = f.user_id
  AND u.status = TRUE AND u.removed_at IS NULL
  AND (sqlc.arg(inactive_days)::int = -1
       OR u.last_active_at > CURRENT_TIMESTAMP - make_interval(days => sqlc.arg(inactive_days)::int))
CROSS JOIN LATERAL jsonb_array_elements_text(f.keywords) AS fk(value)
JOIN jsonb_array_elements_text(sqlc.arg(keywords)::jsonb) AS ak(value) ON ak.value = fk.value
WHERE f.status = TRUE AND f.removed_at IS NULL
  AND f.keywords ?| ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(keywords)::jsonb))
GROUP BY f.id;

-- name: UpdateFeedByIDAndUser :one
UPDATE feeds
SET name = $1, keywords = $2, modified_at = CURRENT_TIMESTAMP
WHERE id = $3 AND user_id = $4 AND status = TRUE AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteFeedByIDAndUser :exec
UPDATE feeds
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $2 AND removed_at IS NULL;

-- name: SoftDeleteFeedsByUser :exec
UPDATE feeds
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND removed_at IS NULL;
