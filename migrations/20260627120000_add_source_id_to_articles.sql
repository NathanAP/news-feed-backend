-- +goose Up
-- Add a NOT NULL source_id (FK to sources) linking each article to its origin source.
-- SQLite rejects ADD COLUMN ... NOT NULL without a non-null default, so the table is rebuilt.
-- There is no retroactive handling: pre-existing articles are not expected (fresh start).

CREATE TABLE articles_new (
    id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    url_original TEXT NOT NULL,
    keywords TEXT NOT NULL DEFAULT '[]',
    source_id TEXT NOT NULL REFERENCES sources(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id)
);

INSERT INTO articles_new (id, status, title, content, url_original, keywords, created_at, modified_at, removed_at)
SELECT id, status, title, content, url_original, keywords, created_at, modified_at, removed_at FROM articles;

DROP TABLE articles;

ALTER TABLE articles_new RENAME TO articles;

CREATE UNIQUE INDEX idx_articles_url_original_active ON articles(url_original) WHERE removed_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_articles_url_original_active;

CREATE TABLE articles_old (
    id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    url_original TEXT NOT NULL,
    keywords TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id)
);

INSERT INTO articles_old (id, status, title, content, url_original, keywords, created_at, modified_at, removed_at)
SELECT id, status, title, content, url_original, keywords, created_at, modified_at, removed_at FROM articles;

DROP TABLE articles;

ALTER TABLE articles_old RENAME TO articles;

CREATE UNIQUE INDEX idx_articles_url_original_active ON articles(url_original) WHERE removed_at IS NULL;
