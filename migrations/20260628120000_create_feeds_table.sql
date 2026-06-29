-- +goose Up
CREATE TABLE IF NOT EXISTS feeds (
    id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    name TEXT NOT NULL,
    keywords TEXT NOT NULL DEFAULT '[]',
    user_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- +goose Down
DROP TABLE IF EXISTS feeds;
