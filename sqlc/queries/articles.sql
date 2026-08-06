-- name: CreateArticle :one
INSERT INTO articles (id, title, content, url_original, keywords, source_id, language_original)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: FindArticleByID :one
SELECT * FROM articles
WHERE id = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: SuggestPopularKeywords :many
-- Keyword suggestions, "popular" strategy: the keywords carried by the most active articles inside a
-- recency window, for the feed-building keyword-suggestion endpoint. This is the fallback path (used
-- when nothing is selected yet, or when the "related" query came back empty), so it must always be
-- able to return something as long as the window holds any article.
-- LATERAL expands each active article's keyword array into rows; COUNT per distinct keyword is how
-- many articles carry it (keywords are unique within an article, so one row per article-keyword).
-- The window (created_at >= since) is what makes "popular" mean "popular right now" and keeps the
-- full-table aggregation bounded; the caller passes the epoch when the window is disabled.
-- exclude is the caller's already-picked keywords as a JSON array (empty = exclude nothing, since
-- value <> ALL(empty) is TRUE); no point suggesting a keyword the user already has.
-- Ordered by count, then keyword for a stable, deterministic tie-break (same lesson as the 0.38 lists).
SELECT kw.value::text AS keyword, COUNT(*) AS occurrences
FROM articles a
CROSS JOIN LATERAL jsonb_array_elements_text(a.keywords) AS kw(value)
WHERE a.status = TRUE AND a.removed_at IS NULL
  AND a.created_at >= sqlc.arg(since)::timestamptz
  AND kw.value <> ALL(ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(exclude)::jsonb)))
GROUP BY kw.value
ORDER BY occurrences DESC, keyword ASC
LIMIT sqlc.arg(result_limit);

-- name: SuggestRelatedKeywords :many
-- Keyword suggestions, "related" strategy: keywords that co-occur with the ones the user already
-- picked. Narrows to articles that carry ANY selected keyword (keywords ?| selected), then counts
-- the OTHER keywords those articles carry. Deliberately not windowed (unlike popular): relatedness is
-- topical, not temporal, and a window would only make the empty-result fallback fire more often for a
-- niche pick.
-- The `?|` predicate is what reaches idx_articles_keywords (GIN): it is applied as a row filter that
-- narrows which articles get expanded, exactly the shape the 0.37.3 review established. Keep it in
-- sync with that index. selected drives both the narrowing (which articles) and the exclusion (never
-- suggest back a keyword the user already picked).
--
-- selected is a text[] and NOT a JSON array, and that is a planner decision, not a style one (0.46.4).
-- Reaching a GIN index needs more than the right operator: the right-hand side must also be something
-- the planner can estimate. Written as `?| ARRAY(SELECT jsonb_array_elements_text($1::jsonb))` the
-- operand is an InitPlan, opaque at plan time, so the planner falls back to a default selectivity and
-- prices the index above a seq scan. Measured on 20k articles: the ARRAY(SELECT ...) form planned a
-- Seq Scan (cost 677, 19999 rows discarded by filter) while `?| $1::text[]` planned a Bitmap Index
-- Scan (cost 433). Forcing enable_seqscan=off proved the index was usable either way - the planner
-- simply would not choose it. Do not "simplify" this back into a jsonb subquery.
SELECT kw.value::text AS keyword, COUNT(*) AS occurrences
FROM articles a
CROSS JOIN LATERAL jsonb_array_elements_text(a.keywords) AS kw(value)
WHERE a.status = TRUE AND a.removed_at IS NULL
  AND a.keywords ?| sqlc.arg(selected)::text[]
  AND kw.value <> ALL(sqlc.arg(selected)::text[])
GROUP BY kw.value
ORDER BY occurrences DESC, keyword ASC
LIMIT sqlc.arg(result_limit);

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

-- name: UpdateArticleContent :exec
-- Overwrites only the body, for the 0.45 treatment DB step: right after an article is stored, its
-- anchors are rewritten to outbound-link ids and the body is written back. Keeps title/keywords/etc.
-- untouched (unlike UpdateArticle). Runs inside the same transaction as the insert.
UPDATE articles
SET content = $1, modified_at = CURRENT_TIMESTAMP
WHERE id = $2 AND status = TRUE AND removed_at IS NULL;

-- name: SoftDeleteArticle :exec
UPDATE articles
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE id = $1 AND removed_at IS NULL;

-- name: SoftDeleteArticlesBySourceID :exec
UPDATE articles
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE source_id = $1 AND removed_at IS NULL;
