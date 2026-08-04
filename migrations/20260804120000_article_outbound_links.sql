-- +goose Up
-- 0.45: outbound-link table. It decouples the link *target* an article's body points at from the body
-- itself. Every <a href> in a treated article stops carrying a URL and instead carries the id of a row
-- in this table; the row's `href` holds the real URL (internal or external). Serving an article swaps
-- the id back to the row's href (a read-time step, on the hot read paths — accepted trade-off for the
-- retroactivity below).
--
-- Why a table instead of rewriting the body: an already-stored body is never rescanned (too expensive
-- per discovery run). So when article B arrives later carrying a url_original that an older article A
-- had linked to externally, A's body cannot be touched. Instead A's link lives here as a row, and B's
-- arrival just UPDATEs that row's href to point at B internally. The body never changes; the pointer does.
--
-- Internal links are stored as the literal token `{CLIENT_URL}/articles/{id}` (not the expanded URL),
-- so a CLIENT_URL change is an env change, not a data migration; the read-time swap expands the token.
-- External links are stored literally.
--
-- No status/removed_at (registered as an exception in conventions.md): these are not soft-deletable
-- records, they are pointers owned by an article. Cascade is hard-delete keyed by article_id, mirroring
-- an article hard-remove (dormant today — articles are only ever soft-removed — but ready in code).
--
-- No backfill: the table starts empty. Articles stored before 0.45 keep real URLs in their body; the
-- read swap only replaces hrefs that match an outbound id, so a legacy (non-id) href passes through
-- untouched. A clean reseed is enough to move dev to the new format.
CREATE TABLE article_outbound_links (
    id TEXT NOT NULL,
    article_id TEXT NOT NULL REFERENCES articles(id),
    href TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    PRIMARY KEY (id)
);

-- article_id: the read-time swap fetches every link of the article(s) on a page in one batch (ANY).
CREATE INDEX idx_article_outbound_links_article_id ON article_outbound_links(article_id);

-- href: the retroactive retarget (WHERE href = url_original) and the pre-remove cleanup
-- (WHERE href = {CLIENT_URL}/articles/{id}) both look a row up by its exact href value.
CREATE INDEX idx_article_outbound_links_href ON article_outbound_links(href);

-- +goose Down
DROP TABLE article_outbound_links;
