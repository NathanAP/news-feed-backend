-- +goose Up
CREATE TABLE IF NOT EXISTS system (
    id TEXT NOT NULL,
    app_status INTEGER NOT NULL DEFAULT 1,
    last_article_discovery_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME,
    PRIMARY KEY (id)
);

-- Seed the single control-panel row. This table is a singleton: application code never
-- inserts or deletes rows, it only updates this one. The id below is a fixed UUID v7 literal
-- because SQLite has no native UUID v7 function — the same accepted, migration-time-only
-- exception used for the user_preferences backfill. app_status starts active (1) so the API
-- works out of the box; last_article_discovery_at starts NULL because discovery (0.19.0.0)
-- has never run yet.
INSERT INTO system (id, app_status, last_article_discovery_at)
VALUES ('01900000-0000-7000-8000-000000000001', 1, NULL);

-- +goose Down
DROP TABLE IF EXISTS system;
