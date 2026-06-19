-- +goose Up
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
