-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id TEXT NOT NULL,
    google_id TEXT NOT NULL,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    picture TEXT,
    status INTEGER NOT NULL DEFAULT 1,
    last_login_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id),
    UNIQUE (google_id),
    UNIQUE (email)
);

-- +goose Down
DROP TABLE IF EXISTS users;
