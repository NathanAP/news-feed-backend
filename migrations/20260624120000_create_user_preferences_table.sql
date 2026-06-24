-- +goose Up
CREATE TABLE IF NOT EXISTS user_preferences (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    theme TEXT NOT NULL DEFAULT 'dark',
    language TEXT NOT NULL DEFAULT 'pt',
    translate_content INTEGER NOT NULL DEFAULT 1,
    ai_personality TEXT NOT NULL DEFAULT 'mixed',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    UNIQUE (user_id)
);

INSERT INTO user_preferences (id, user_id, status, theme, language, translate_content, ai_personality, created_at)
SELECT
    lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))),
    id, 1, 'dark', 'pt', 1, 'mixed', CURRENT_TIMESTAMP
FROM users WHERE removed_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS user_preferences;
