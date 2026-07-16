-- +goose Up
-- 0.37: initial schema on PostgreSQL 18. This single migration replaces the 13 SQLite migrations
-- that preceded it. The old ones were dropped rather than ported: no environment was ever deployed
-- and all data was disposable, so there is no history worth reconstructing and no ETL to run.
--
-- What changed versus the SQLite schema, and why:
--   * status / is_read / app_status: INTEGER (0/1) -> BOOLEAN. SQLite has no boolean type, so the
--     convention's "status true/false" had to be emulated with 0/1 and leaked into Go as int64.
--   * DATETIME -> TIMESTAMPTZ. The project stores every date in UTC (conventions.md); TIMESTAMPTZ
--     makes the database itself enforce that instead of relying on the caller.
--   * keywords: TEXT holding a JSON array -> JSONB. It is queried as a JSON array by the judgement
--     layer-1 overlap, so the real type buys both correctness and a GIN index.
--   * id stays TEXT (UUID v7 generated in Go). Moving to the native uuid type is a separate version:
--     it would ripple through every model, controller, fixture and mock.
--
-- Partial unique indexes are declared inline here: PostgreSQL supports them natively and supports
-- ALTER TABLE, so the table-rebuild dance the SQLite migrations needed simply does not exist.

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
-- This GIN index is what keeps that from scanning the whole feed table; it did not exist under
-- SQLite, where keywords was opaque TEXT.
--
-- Careful: a GIN index is only reachable through one of its OPERATORS (@>, ?, ?|, ?&). Expanding
-- the column with jsonb_array_elements_text (as the overlap COUNT must) is a function call on every
-- row and can never use it. FindCandidateFeedsByKeywords therefore carries an explicit `keywords ?|`
-- predicate alongside the expansion - if that predicate is ever dropped, this index silently stops
-- being used and layer 1 goes back to a full scan (0.37.3.0 fixed exactly that).
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
-- soft-deletable record; the switch itself is app_status.
CREATE TABLE system (
    id TEXT NOT NULL,
    app_status BOOLEAN NOT NULL DEFAULT TRUE,
    last_article_discovery_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at TIMESTAMPTZ,
    PRIMARY KEY (id)
);

-- Seed the single control-panel row. The id is a fixed UUID v7 literal on purpose: this row is a
-- singleton and every environment must agree on its id, so it must be deterministic rather than
-- generated (PostgreSQL 18 does ship a native uuidv7(), but a random id here would defeat the
-- point). app_status starts active so the API works out of the box; last_article_discovery_at
-- starts NULL because discovery has never run on a fresh database.
INSERT INTO system (id, app_status, last_article_discovery_at)
VALUES ('01900000-0000-7000-8000-000000000001', TRUE, NULL);

-- +goose Down
-- Dropped in reverse dependency order: junction first, then the tables it points at.
DROP TABLE IF EXISTS articles_feeds;
DROP TABLE IF EXISTS feeds;
DROP TABLE IF EXISTS articles;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS system;
DROP TABLE IF EXISTS users;
