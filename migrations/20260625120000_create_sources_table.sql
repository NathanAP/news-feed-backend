-- +goose Up
CREATE TABLE IF NOT EXISTS sources (
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

-- +goose Down
DROP TABLE IF EXISTS sources;
