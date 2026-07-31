-- Mirror of the schema produced by migrations/, consumed by sqlc to type the queries.
-- Keep in sync with everything under migrations/ — this file is not executed against any database;
-- it is only sqlc's source of truth for column types.
--
-- Column ORDER matters here, not just the set of columns: the queries use SELECT * / RETURNING *, so
-- sqlc generates the Scan in the order declared below and the driver returns them in the order the
-- real table has. A column added by a later migration therefore has to be appended at the end of the
-- table here, exactly where ALTER TABLE ... ADD COLUMN puts it — never inserted in the middle where
-- it reads more naturally.

CREATE TABLE users (
    id TEXT NOT NULL,
    google_id TEXT NOT NULL,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    picture TEXT,
    status BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    -- Appended by migrations/20260729120000_users_admin_flag.sql (0.40). Stays last: see the note above.
    admin BOOLEAN NOT NULL DEFAULT FALSE,
    -- Appended by migrations/20260731120000_users_last_active_at.sql (0.43). Stays last: see the note above.
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE (google_id),
    UNIQUE (email)
);

CREATE TABLE user_preferences (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status BOOLEAN NOT NULL DEFAULT TRUE,
    language_to_translate TEXT,
    ai_personality TEXT NOT NULL DEFAULT 'mixed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    UNIQUE (user_id)
);

CREATE TABLE refresh_tokens (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE sources (
    id TEXT NOT NULL,
    status BOOLEAN NOT NULL DEFAULT TRUE,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    url_rss TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    PRIMARY KEY (id)
);

-- Uniqueness applies only to active rows: a soft-deleted source must not block creating
-- a new source with the same url/url_rss (status convention).
CREATE UNIQUE INDEX idx_sources_url_active ON sources(url) WHERE removed_at IS NULL;
CREATE UNIQUE INDEX idx_sources_url_rss_active ON sources(url_rss) WHERE removed_at IS NULL;

CREATE TABLE articles (
    id TEXT NOT NULL,
    status BOOLEAN NOT NULL DEFAULT TRUE,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    url_original TEXT NOT NULL,
    keywords JSONB NOT NULL DEFAULT '[]'::JSONB,
    source_id TEXT NOT NULL REFERENCES sources(id),
    language_original TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    PRIMARY KEY (id)
);

-- Uniqueness applies only to active rows: a soft-deleted article must not block creating
-- a new article with the same url_original (status convention).
CREATE UNIQUE INDEX idx_articles_url_original_active ON articles(url_original) WHERE removed_at IS NULL;

-- Keyword-suggestion "related" strategy narrows articles with `keywords ?|` before counting the
-- co-occurring keywords. Reachable only through a GIN operator (see idx_feeds_keywords for the same
-- reasoning on the feeds side). SuggestRelatedKeywords must carry a `?|` predicate to reach it.
CREATE INDEX idx_articles_keywords ON articles USING GIN (keywords);

CREATE TABLE feeds (
    id TEXT NOT NULL,
    status BOOLEAN NOT NULL DEFAULT TRUE,
    name TEXT NOT NULL,
    keywords JSONB NOT NULL DEFAULT '[]'::JSONB,
    user_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Judgement layer 1 looks for keyword overlap between an incoming article and every active feed.
-- Only reachable through a GIN operator (@>, ?, ?|, ?&): FindCandidateFeedsByKeywords carries an
-- explicit `keywords ?|` predicate for this, since the jsonb_array_elements_text expansion the
-- overlap COUNT needs is a per-row function call that no index can serve.
CREATE INDEX idx_feeds_keywords ON feeds USING GIN (keywords);

CREATE TABLE articles_feeds (
    id TEXT NOT NULL,
    article_id TEXT NOT NULL,
    feed_id TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    PRIMARY KEY (id),
    FOREIGN KEY (article_id) REFERENCES articles(id),
    FOREIGN KEY (feed_id) REFERENCES feeds(id)
);

-- Singleton control-panel table: exactly one row, only ever updated (never inserted into or
-- deleted from by application code). No status/removed_at — a maintenance switch is not a
-- soft-deletable record; the switch itself is app_status. The row itself is seeded by the migration.
CREATE TABLE system (
    id TEXT NOT NULL,
    app_status BOOLEAN NOT NULL DEFAULT TRUE,
    last_article_discovery_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    PRIMARY KEY (id)
);
