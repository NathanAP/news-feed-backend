-- +goose Up
-- 0.33: user preferences slimmed down. `theme` moves to the client (localStorage), and
-- `translate_content` is folded into a single nullable `language_to_translate` (renamed from
-- `language`): a null value means the client hides the translation option entirely. SQLite cannot
-- drop a column's NOT NULL/DEFAULT in place, so the table is rebuilt and the data copied over
-- (the old `language` maps straight to `language_to_translate`; `theme`/`translate_content` are dropped).
CREATE TABLE user_preferences_new (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    language_to_translate TEXT,
    ai_personality TEXT NOT NULL DEFAULT 'mixed',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    UNIQUE (user_id)
);

INSERT INTO user_preferences_new (id, user_id, status, language_to_translate, ai_personality, created_at, modified_at, removed_at)
SELECT id, user_id, status, language, ai_personality, created_at, modified_at, removed_at
FROM user_preferences;

DROP TABLE user_preferences;
ALTER TABLE user_preferences_new RENAME TO user_preferences;

-- +goose Down
-- Reverse the rebuild: restore `theme` (default 'dark'), `translate_content` (default 1) and the
-- NOT NULL `language` column, coalescing a null language_to_translate back to the old 'pt' default.
CREATE TABLE user_preferences_old (
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

INSERT INTO user_preferences_old (id, user_id, status, theme, language, translate_content, ai_personality, created_at, modified_at, removed_at)
SELECT id, user_id, status, 'dark', COALESCE(language_to_translate, 'pt'), 1, ai_personality, created_at, modified_at, removed_at
FROM user_preferences;

DROP TABLE user_preferences;
ALTER TABLE user_preferences_old RENAME TO user_preferences;
