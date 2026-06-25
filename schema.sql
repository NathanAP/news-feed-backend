CREATE TABLE users (
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

CREATE TABLE user_preferences (
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

CREATE TABLE refresh_tokens (
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

CREATE TABLE sources (
    id TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    url TEXT NOT NULL,
    url_rss TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    removed_at DATETIME,
    PRIMARY KEY (id)
);

-- Uniqueness applies only to active rows: a soft-deleted source must not block creating
-- a new source with the same url/url_rss (status convention).
CREATE UNIQUE INDEX idx_sources_url_active ON sources(url) WHERE removed_at IS NULL;
CREATE UNIQUE INDEX idx_sources_url_rss_active ON sources(url_rss) WHERE removed_at IS NULL;
