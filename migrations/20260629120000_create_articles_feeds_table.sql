-- +goose Up
CREATE TABLE IF NOT EXISTS articles_feeds (
    id TEXT NOT NULL,
    article_id TEXT NOT NULL,
    feed_id TEXT NOT NULL,
    is_read INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    PRIMARY KEY (id),
    FOREIGN KEY (article_id) REFERENCES articles(id),
    FOREIGN KEY (feed_id) REFERENCES feeds(id)
);

-- +goose Down
DROP TABLE IF EXISTS articles_feeds;
