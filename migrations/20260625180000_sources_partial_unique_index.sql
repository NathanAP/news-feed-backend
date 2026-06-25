-- +goose Up
-- Replace the table-level UNIQUE constraints on url/url_rss with partial unique indexes
-- scoped to active rows (removed_at IS NULL). The inline constraints enforced uniqueness
-- against ALL rows, including soft-deleted ones, which made a removed source block the
-- creation of a new source with the same url/url_rss — a violation of the status convention
-- (inactive records must never affect active operations). SQLite cannot drop an inline
-- constraint, so the table is rebuilt without it.

CREATE TABLE sources_new (
    id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
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
    PRIMARY KEY (id),
    UNIQUE (url),
    UNIQUE (url_rss)
);

INSERT INTO sources_old (id, status, url, url_rss, created_at, modified_at, removed_at)
SELECT id, status, url, url_rss, created_at, modified_at, removed_at FROM sources;

DROP TABLE sources;

ALTER TABLE sources_old RENAME TO sources;
