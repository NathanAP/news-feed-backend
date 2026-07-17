-- +goose Up
-- 0.39: GIN index on articles.keywords, for the keyword-suggestion endpoint.
--
-- The "related" suggestion strategy narrows articles with `keywords ?| $selected` ("does this
-- article carry ANY of the keywords the user already picked?") before expanding and counting the
-- rest. Without this index that predicate is a sequential scan over every active article.
--
-- The operator matters, not the column: an index is reached by OPERATOR, and `?|` is exactly what
-- jsonb_ops (the default GIN opclass) serves. This is the lesson from the 0.37.3 review, where
-- idx_feeds_keywords existed but the layer-1 judgement query expanded the column through
-- jsonb_array_elements_text instead — a function call per row, which no index can serve, so the
-- index was dead weight until a `?|` predicate was added next to it (428ms -> 22ms on 60k feeds).
-- Keep the `?|` predicate in SuggestRelatedKeywords in sync with this index.
--
-- Mirrors idx_feeds_keywords on the feeds side. Not partial (unlike the *_active indexes): jsonb_ops
-- has no notion of the status/removed_at columns, and the status predicate is applied on top of the
-- bitmap the index produces.
CREATE INDEX idx_articles_keywords ON articles USING GIN (keywords);

-- +goose Down
DROP INDEX IF EXISTS idx_articles_keywords;
