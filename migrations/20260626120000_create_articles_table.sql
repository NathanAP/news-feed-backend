-- +goose Up
CREATE TABLE IF NOT EXISTS articles (
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

-- Uniqueness applies only to active rows: a soft-deleted article must not block creating
-- a new article with the same url_original (status convention).
CREATE UNIQUE INDEX idx_articles_url_original_active ON articles(url_original) WHERE removed_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_articles_url_original_active;
DROP TABLE IF EXISTS articles;
