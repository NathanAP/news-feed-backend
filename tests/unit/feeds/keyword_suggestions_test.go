package feeds_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	feedendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/feeds"
	db "github.com/nathanap/news-feed-backend/sqlc"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

// The ranking and the related/popular fallback are proven against a real Postgres in
// tests/integration/api/feeds/keyword_suggestions_test.go. These unit tests cover what the handler
// owns: turning the query string into the right controller arguments (the picked keywords, the
// clamped limit), validation, and shaping the response envelope.

// mockKeywordArticleCtrl is a minimal ArticleControllerInterface exposing only SuggestKeywords; the
// rest return zero values since the keyword-suggestions handler never calls them.
type mockKeywordArticleCtrl struct {
	suggestFn func(ctx context.Context, q db.Querier, selected []string, since time.Time, limit int32) ([]schemas.KeywordSuggestion, string, error)
}

func (m *mockKeywordArticleCtrl) Create(context.Context, db.Querier, string, string, string, string, []string, *string) (db.Article, error) {
	return db.Article{}, nil
}
func (m *mockKeywordArticleCtrl) FindByID(context.Context, db.Querier, string) (db.Article, error) {
	return db.Article{}, nil
}
func (m *mockKeywordArticleCtrl) FindByURLOriginal(context.Context, db.Querier, string) (db.Article, error) {
	return db.Article{}, nil
}
func (m *mockKeywordArticleCtrl) List(context.Context, db.Querier, controllers.ListArticlesFilter) ([]db.Article, int64, error) {
	return nil, 0, nil
}
func (m *mockKeywordArticleCtrl) ListAll(context.Context, db.Querier) ([]db.Article, error) {
	return nil, nil
}
func (m *mockKeywordArticleCtrl) SuggestKeywords(ctx context.Context, q db.Querier, selected []string, since time.Time, limit int32) ([]schemas.KeywordSuggestion, string, error) {
	if m.suggestFn != nil {
		return m.suggestFn(ctx, q, selected, since, limit)
	}
	return []schemas.KeywordSuggestion{}, schemas.KeywordStrategyPopular, nil
}
func (m *mockKeywordArticleCtrl) Update(context.Context, db.Querier, string, string, string, string, []string, *string) (db.Article, error) {
	return db.Article{}, nil
}
func (m *mockKeywordArticleCtrl) SoftDelete(context.Context, db.Querier, string) error { return nil }

func buildKeywordSuggestionsApp(ctrl *mockKeywordArticleCtrl, windowDays int) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	feeds := app.Group("/v1/feeds")
	feeds.Get("/keyword-suggestions", append(authMiddleware, feedendpoints.SuggestKeywords(ctrl, fakeTxRunner, windowDays))...)
	return app
}

func getSuggestions(t *testing.T, app *fiber.App, query string, withAuth bool) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/keyword-suggestions"+query, nil)
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestUnit_KeywordSuggestions_Unauthenticated_401(t *testing.T) {
	requireNotProduction(t)

	app := buildKeywordSuggestionsApp(&mockKeywordArticleCtrl{}, 30)
	resp := getSuggestions(t, app, "", false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// The comma-separated keywords must reach the controller normalized: trimmed, lowercased,
// deduplicated, empties dropped. The handler owns this, since the controller and SQL trust the slice.
func TestUnit_KeywordSuggestions_NormalizesSelectedKeywords(t *testing.T) {
	requireNotProduction(t)

	var got []string
	ctrl := &mockKeywordArticleCtrl{
		suggestFn: func(_ context.Context, _ db.Querier, selected []string, _ time.Time, _ int32) ([]schemas.KeywordSuggestion, string, error) {
			got = selected
			return []schemas.KeywordSuggestion{}, schemas.KeywordStrategyRelated, nil
		},
	}
	app := buildKeywordSuggestionsApp(ctrl, 30)

	resp := getSuggestions(t, app, "?keywords=%20Metallica%20,ROCK,metallica,,rock", true)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{"metallica", "rock"}, got, "trimmed, lowercased, deduped, empties dropped")
}

// No keywords param means "nothing picked yet": the controller must receive an empty slice, which is
// what drives the popular strategy.
func TestUnit_KeywordSuggestions_NoKeywordsMeansEmptySelection(t *testing.T) {
	requireNotProduction(t)

	got := []string{"sentinel"}
	ctrl := &mockKeywordArticleCtrl{
		suggestFn: func(_ context.Context, _ db.Querier, selected []string, _ time.Time, _ int32) ([]schemas.KeywordSuggestion, string, error) {
			got = selected
			return []schemas.KeywordSuggestion{}, schemas.KeywordStrategyPopular, nil
		},
	}
	app := buildKeywordSuggestionsApp(ctrl, 30)

	resp := getSuggestions(t, app, "", true)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, got)
}

// More than a feed's worth of keywords is a 400: the picks can never exceed what a feed could hold.
func TestUnit_KeywordSuggestions_TooManyKeywords_400(t *testing.T) {
	requireNotProduction(t)

	app := buildKeywordSuggestionsApp(&mockKeywordArticleCtrl{}, 30)

	// FeedKeywordsMax + 1 distinct keywords.
	query := "?keywords="
	for i := 0; i <= schemas.FeedKeywordsMax; i++ {
		if i > 0 {
			query += ","
		}
		query += string(rune('a'+i%26)) + string(rune('0'+i/26))
	}

	resp := getSuggestions(t, app, query, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// limit is clamped, never errors: garbage falls back to the default, and values are bounded to
// [1, max]. The clamped value must be what reaches the controller (it becomes the SQL LIMIT).
func TestUnit_KeywordSuggestions_ClampsLimit(t *testing.T) {
	requireNotProduction(t)

	tests := []struct {
		name  string
		query string
		want  int32
	}{
		{name: "default when absent", query: "", want: schemas.KeywordSuggestionsDefaultLimit},
		{name: "explicit value", query: "?limit=5", want: 5},
		{name: "garbage falls back to default", query: "?limit=abc", want: schemas.KeywordSuggestionsDefaultLimit},
		{name: "below min clamps to 1", query: "?limit=0", want: 1},
		{name: "above max clamps to max", query: "?limit=9999", want: schemas.KeywordSuggestionsMaxLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int32
			ctrl := &mockKeywordArticleCtrl{
				suggestFn: func(_ context.Context, _ db.Querier, _ []string, _ time.Time, limit int32) ([]schemas.KeywordSuggestion, string, error) {
					got = limit
					return []schemas.KeywordSuggestion{}, schemas.KeywordStrategyPopular, nil
				},
			}
			app := buildKeywordSuggestionsApp(ctrl, 30)

			resp := getSuggestions(t, app, tt.query, true)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The window is resolved to a concrete "since" from now: with a 30-day window the bound sits ~30 days
// back; with a negative window it is disabled (epoch), so the whole history is in range.
func TestUnit_KeywordSuggestions_ResolvesWindowToSince(t *testing.T) {
	requireNotProduction(t)

	t.Run("positive window is now minus the window", func(t *testing.T) {
		var since time.Time
		ctrl := &mockKeywordArticleCtrl{
			suggestFn: func(_ context.Context, _ db.Querier, _ []string, s time.Time, _ int32) ([]schemas.KeywordSuggestion, string, error) {
				since = s
				return []schemas.KeywordSuggestion{}, schemas.KeywordStrategyPopular, nil
			},
		}
		app := buildKeywordSuggestionsApp(ctrl, 30)

		before := time.Now().UTC().AddDate(0, 0, -30)
		require.Equal(t, http.StatusOK, getSuggestions(t, app, "", true).StatusCode)
		after := time.Now().UTC().AddDate(0, 0, -30)
		assert.False(t, since.Before(before.Add(-time.Minute)))
		assert.False(t, since.After(after.Add(time.Minute)))
	})

	t.Run("negative window disables the bound", func(t *testing.T) {
		var since time.Time
		ctrl := &mockKeywordArticleCtrl{
			suggestFn: func(_ context.Context, _ db.Querier, _ []string, s time.Time, _ int32) ([]schemas.KeywordSuggestion, string, error) {
				since = s
				return []schemas.KeywordSuggestion{}, schemas.KeywordStrategyPopular, nil
			},
		}
		app := buildKeywordSuggestionsApp(ctrl, -1)

		require.Equal(t, http.StatusOK, getSuggestions(t, app, "", true).StatusCode)
		assert.True(t, since.Year() <= 1970, "epoch-ish lower bound means no window")
	})
}

// The response echoes the strategy and the suggestions the controller produced, as the documented
// envelope. Suggestions is always an array, never null.
func TestUnit_KeywordSuggestions_ShapesEnvelope(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockKeywordArticleCtrl{
		suggestFn: func(context.Context, db.Querier, []string, time.Time, int32) ([]schemas.KeywordSuggestion, string, error) {
			return []schemas.KeywordSuggestion{
				{Keyword: "rock", Count: 812},
				{Keyword: "metal", Count: 511},
			}, schemas.KeywordStrategyRelated, nil
		},
	}
	app := buildKeywordSuggestionsApp(ctrl, 30)

	resp := getSuggestions(t, app, "?keywords=metallica", true)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body schemas.KeywordSuggestionsResponse
	require.NoError(t, readJSON(resp, &body))
	assert.Equal(t, schemas.KeywordStrategyRelated, body.Strategy)
	require.Len(t, body.Suggestions, 2)
	assert.Equal(t, "rock", body.Suggestions[0].Keyword)
	assert.Equal(t, int64(812), body.Suggestions[0].Count)
}

func TestUnit_KeywordSuggestions_ControllerError_500(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockKeywordArticleCtrl{
		suggestFn: func(context.Context, db.Querier, []string, time.Time, int32) ([]schemas.KeywordSuggestion, string, error) {
			return nil, "", assert.AnError
		},
	}
	app := buildKeywordSuggestionsApp(ctrl, 30)

	resp := getSuggestions(t, app, "", true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
