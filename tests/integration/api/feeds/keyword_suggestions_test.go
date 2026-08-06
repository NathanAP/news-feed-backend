package feeds_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/schemas"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// The ranking, the related/popular fallback, and the selected-keyword exclusion all live in SQL, so
// they can only be proven against a real Postgres. The unit tests only prove the handler forwards the
// right arguments. The integration setup registers keyword-suggestions with the window disabled, so
// created_at never has to be forced here.

// seedKeywordedArticle inserts an active article carrying the given keywords, straight through the
// querier (suggestions draw from the global article pool, so the article need not belong to a feed).
func seedKeywordedArticle(t *testing.T, queries db.Querier, sourceID, id string, keywords []string) {
	t.Helper()
	kw, err := json.Marshal(keywords)
	require.NoError(t, err)
	_, err = queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID:          id,
		Title:       "Article " + id,
		Content:     "content",
		UrlOriginal: "https://articles.example.com/" + id,
		Keywords:    kw,
		SourceID:    sourceID,
	})
	require.NoError(t, err)
}

func getSuggestions(t *testing.T, app *fiber.App, token, query string) schemas.KeywordSuggestionsResponse {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/keyword-suggestions"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body schemas.KeywordSuggestionsResponse
	require.NoError(t, readJSON(resp, &body))
	return body
}

func keywordsOf(suggestions []schemas.KeywordSuggestion) []string {
	out := make([]string, len(suggestions))
	for i, s := range suggestions {
		out[i] = s.Keyword
	}
	return out
}

// With nothing picked, the strategy is "popular": the keywords ranked by how many articles carry them.
func TestIntegration_KeywordSuggestions_PopularWhenNothingSelected(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "80")
	sourceID := seedSource(t, queries, "80")

	seedKeywordedArticle(t, queries, sourceID, "a1", []string{"rock", "metal"})
	seedKeywordedArticle(t, queries, sourceID, "a2", []string{"rock", "jazz"})
	seedKeywordedArticle(t, queries, sourceID, "a3", []string{"rock", "metal"})

	body := getSuggestions(t, app, token, "")
	assert.Equal(t, schemas.KeywordStrategyPopular, body.Strategy)
	// rock in 3, metal in 2, jazz in 1.
	assert.Equal(t, []string{"rock", "metal", "jazz"}, keywordsOf(body.Suggestions))
	assert.Equal(t, int64(3), body.Suggestions[0].Count)
}

// With picks, the strategy is "related": keywords that co-occur with the picks, ranked by count, and
// the picked keyword is never suggested back.
func TestIntegration_KeywordSuggestions_RelatedWhenSelected(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "81")
	sourceID := seedSource(t, queries, "81")

	seedKeywordedArticle(t, queries, sourceID, "b1", []string{"metallica", "rock", "thrash"})
	seedKeywordedArticle(t, queries, sourceID, "b2", []string{"metallica", "rock", "albums"})
	seedKeywordedArticle(t, queries, sourceID, "b3", []string{"jazz", "smooth"}) // unrelated

	body := getSuggestions(t, app, token, "?keywords=metallica")
	assert.Equal(t, schemas.KeywordStrategyRelated, body.Strategy)

	got := keywordsOf(body.Suggestions)
	assert.NotContains(t, got, "metallica", "never suggest back a picked keyword")
	assert.NotContains(t, got, "jazz", "a keyword from an unrelated article must not appear")
	// rock co-occurs twice; albums and thrash once each, broken alphabetically.
	assert.Equal(t, []string{"rock", "albums", "thrash"}, got)
	assert.Equal(t, int64(2), body.Suggestions[0].Count)
}

// A pick that co-occurs with nothing yields an empty "related", which must fall back to "popular"
// rather than returning an empty list — the endpoint is never without suggestions (roadmap).
func TestIntegration_KeywordSuggestions_FallsBackToPopular(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "82")
	sourceID := seedSource(t, queries, "82")

	seedKeywordedArticle(t, queries, sourceID, "c1", []string{"rock", "metal"})
	seedKeywordedArticle(t, queries, sourceID, "c2", []string{"rock", "pop"})

	// "isolated" is not in any article, so nothing co-occurs with it.
	body := getSuggestions(t, app, token, "?keywords=isolated")
	assert.Equal(t, schemas.KeywordStrategyPopular, body.Strategy, "empty related must fall back to popular")
	assert.Contains(t, keywordsOf(body.Suggestions), "rock")
}

// Even the popular fallback excludes the picked keywords. To reach the fallback while the pick still
// exists in the pool, the only article carrying the pick has no OTHER keyword — so "related" is empty
// (nothing co-occurs) and we fall to popular, where the pick is present but must still be filtered out.
func TestIntegration_KeywordSuggestions_FallbackStillExcludesSelected(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "83")
	sourceID := seedSource(t, queries, "83")

	seedKeywordedArticle(t, queries, sourceID, "d0", []string{"solo"}) // pick exists, but co-occurs with nothing
	seedKeywordedArticle(t, queries, sourceID, "d1", []string{"rock", "metal"})
	seedKeywordedArticle(t, queries, sourceID, "d2", []string{"rock", "blues"})

	body := getSuggestions(t, app, token, "?keywords=solo")
	require.Equal(t, schemas.KeywordStrategyPopular, body.Strategy, "solo co-occurs with nothing, so related is empty")
	got := keywordsOf(body.Suggestions)
	assert.Contains(t, got, "rock", "popular still returns the actual top keywords")
	assert.NotContains(t, got, "solo", "a picked keyword is excluded even in the fallback")
}

// An empty database is a legitimate empty result, not an error: strategy popular, suggestions [] (not
// null), status 200.
func TestIntegration_KeywordSuggestions_EmptyDatabase(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "84")

	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/keyword-suggestions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Assert the raw JSON carries [] rather than null before decoding.
	var raw map[string]json.RawMessage
	require.NoError(t, readJSON(resp, &raw))
	assert.Equal(t, "[]", string(raw["suggestions"]))
}

// limit bounds the number of suggestions returned.
func TestIntegration_KeywordSuggestions_RespectsLimit(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "85")
	sourceID := seedSource(t, queries, "85")

	seedKeywordedArticle(t, queries, sourceID, "e1", []string{"one", "two", "three", "four", "five"})

	body := getSuggestions(t, app, token, "?limit=2")
	assert.Len(t, body.Suggestions, 2)
}

// Soft-deleted articles must not feed the counts: an inactive article's keywords must be invisible to
// suggestions, matching the status rules everywhere else.
func TestIntegration_KeywordSuggestions_ExcludesSoftDeletedArticles(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "86")
	sourceID := seedSource(t, queries, "86")

	seedKeywordedArticle(t, queries, sourceID, "f1", []string{"kept"})
	seedKeywordedArticle(t, queries, sourceID, "f2", []string{"gone"})
	require.NoError(t, queries.SoftDeleteArticle(t.Context(), "f2"))

	got := keywordsOf(getSuggestions(t, app, token, "").Suggestions)
	assert.Contains(t, got, "kept")
	assert.NotContains(t, got, "gone", "a soft-deleted article's keyword must not be suggested")
}

func TestIntegration_KeywordSuggestions_Unauthenticated_401(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/keyword-suggestions", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
