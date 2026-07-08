-- +goose Up
-- Add a NOT NULL name to sources (display name of the news source, e.g. "G1", "BBC"), so the
-- client can show it without deriving one from the url. SQLite rejects ADD COLUMN ... NOT NULL
-- without a non-null default, so the table is rebuilt. There is no retroactive handling:
-- pre-existing sources are not expected (fresh start).

CREATE TABLE sources_new (
    id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    url_rss TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id)
);

INSERT INTO sources_new (id, status, url, url_rss, created_at, modified_at, removed_at)
SELECT id, status, url, url_rss, created_at, modified_at, removed_at FROM sources;

DROP TABLE sources;

ALTER TABLE sources_new RENAME TO sources;

CREATE UNIQUE INDEX idx_sources_url_active ON sources(url) WHERE removed_at IS NULL;
CREATE UNIQUE INDEX idx_sources_url_rss_active ON sources(url_rss) WHERE removed_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_sources_url_active;
DROP INDEX IF EXISTS idx_sources_url_rss_active;

CREATE TABLE sources_old (
    id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    url TEXT NOT NULL,
    url_rss TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id)
);

INSERT INTO sources_old (id, status, url, url_rss, created_at, modified_at, removed_at)
SELECT id, status, url, url_rss, created_at, modified_at, removed_at FROM sources;

DROP TABLE sources;

ALTER TABLE sources_old RENAME TO sources;

CREATE UNIQUE INDEX idx_sources_url_active ON sources(url) WHERE removed_at IS NULL;
CREATE UNIQUE INDEX idx_sources_url_rss_active ON sources(url_rss) WHERE removed_at IS NULL;
