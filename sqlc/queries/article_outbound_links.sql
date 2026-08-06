-- name: CreateArticleOutboundLink :one
-- One row per distinct href found in a freshly persisted article body (see the outboundlinks service).
-- href is the real target: an internal link as the literal token `{CLIENT_URL}/articles/{id}`, an
-- external link literally. The body stores this row's id in the anchor, never the URL.
INSERT INTO article_outbound_links (id, article_id, href)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListArticleOutboundLinksByArticleIDs :many
-- Every outbound link of the given articles, in one batch, for the read-time swap. The listing/read
-- endpoints pass the ids of the whole page here (never one query per article - no N+1).
-- Stays on ANY(...::text[]) even though it makes sqlc emit pq.Array, which is the only reason lib/pq
-- is still a dependency alongside the real driver (pgx). Do NOT "clean this up" with sqlc.slice:
-- tried in 0.46 and it is broken for the postgresql engine - sqlc renders the placeholder as `$1`,
-- losing the /*SLICE:*/ marker its own generated strings.Replace looks for, and then builds MySQL
-- style `?` placeholders. Result compiles and passes any test that sends a single id, but fails at
-- runtime as soon as a page carries two (bind message supplies N parameters, statement requires 1).
SELECT * FROM article_outbound_links
WHERE article_id = ANY(sqlc.arg(article_ids)::text[]);

-- name: RetargetArticleOutboundLinksByHref :exec
-- Repoints every row currently pointing at old_href to new_href. Two callers, both matching by exact
-- href (idx_article_outbound_links_href):
--   * retroactive linking: when article B is persisted, old_href = B.url_original, new_href =
--     `{CLIENT_URL}/articles/{B.id}` — older articles that linked B's external URL now open internally.
--   * pre-remove cleanup: before an article L is removed, old_href = `{CLIENT_URL}/articles/{L.id}`,
--     new_href = L.url_original — links pointing at the vanishing internal page fall back to the source.
UPDATE article_outbound_links
SET href = sqlc.arg(new_href), modified_at = CURRENT_TIMESTAMP
WHERE href = sqlc.arg(old_href);

-- name: DeleteArticleOutboundLinksByArticleID :exec
-- Hard-delete cascade for an article hard-remove. Dormant today (articles are only soft-removed) but
-- kept ready in code, per the project's cascade philosophy.
DELETE FROM article_outbound_links
WHERE article_id = $1;

-- name: RetargetArticleOutboundLinksBySourceID :exec
-- Cascade counterpart of RetargetArticleOutboundLinksByHref, for the source soft-delete (0.48.3).
-- Soft-deleting a source cascades to its articles, and a cascaded article is still a removed article:
-- every outbound link pointing at its internal page has to fall back to the source URL, or older
-- articles keep anchors to a page that answers 404. The single-article delete path had this since
-- 0.46.1; the cascade path did not, which is the gap this closes.
--
-- Set-based on purpose: one statement instead of one Retarget per article. A source can own a lot of
-- articles, and an N+1 inside the deletion transaction is the wrong shape.
--
-- internal_href_prefix is `{CLIENT_URL}/articles/` supplied by Go (outboundlinks.InternalHrefPrefix),
-- so the token format stays owned by the outboundlinks package instead of being duplicated in SQL.
UPDATE article_outbound_links l
SET href = a.url_original, modified_at = CURRENT_TIMESTAMP
FROM articles a
WHERE a.source_id = sqlc.arg(source_id)
  AND a.removed_at IS NULL
  AND l.href = sqlc.arg(internal_href_prefix)::text || a.id;
