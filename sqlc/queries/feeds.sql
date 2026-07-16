-- name: CreateFeed :one
INSERT INTO feeds (id, name, keywords, user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: FindFeedByIDAndUser :one
SELECT * FROM feeds
WHERE id = $1 AND user_id = $2 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: ListFeedsByUser :many
SELECT * FROM feeds
WHERE user_id = $1 AND status = TRUE AND removed_at IS NULL
ORDER BY created_at DESC;

-- name: CountActiveFeedsByUser :one
SELECT COUNT(*) FROM feeds
WHERE user_id = $1 AND status = TRUE AND removed_at IS NULL;

-- name: FindCandidateFeedsByKeywords :many
-- Judgement layer 1 (keyword overlap): returns every active feed (of any user) that shares at
-- least one keyword with the article, along with overlap_count (how many distinct keywords matched).
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
