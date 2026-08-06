package feeds_test

import (
	"context"
	"encoding/json"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	feedendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/feeds"
	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

// mockAFCtrl is a minimal ArticleFeedControllerInterface for the feed-articles handler: only the
// list method is exercised here; the rest return zero values.
type mockAFCtrl struct {
	listByFeedFn  func(ctx context.Context, q db.Querier, feedID, userID string, filter controllers.ListFeedArticlesFilter) ([]db.ListArticlesByFeedForUserRow, int64, error)
	countUnreadFn func(ctx context.Context, q db.Querier, userID string) ([]db.CountUnreadArticlesByFeedForUserRow, error)
}

func (m *mockAFCtrl) Create(_ context.Context, _ db.Querier, _, _ string) (db.ArticlesFeed, error) {
	return db.ArticlesFeed{}, nil
}
func (m *mockAFCtrl) FindByArticleAndUser(_ context.Context, _ db.Querier, _, _ string) ([]db.ArticlesFeed, error) {
	return []db.ArticlesFeed{}, nil
}
func (m *mockAFCtrl) ListArticlesByFeedForUser(ctx context.Context, q db.Querier, feedID, userID string, filter controllers.ListFeedArticlesFilter) ([]db.ListArticlesByFeedForUserRow, int64, error) {
	if m.listByFeedFn != nil {
		return m.listByFeedFn(ctx, q, feedID, userID, filter)
	}
	return []db.ListArticlesByFeedForUserRow{}, 0, nil
}
func (m *mockAFCtrl) CountUnreadByFeedForUser(ctx context.Context, q db.Querier, userID string) ([]db.CountUnreadArticlesByFeedForUserRow, error) {
	if m.countUnreadFn != nil {
		return m.countUnreadFn(ctx, q, userID)
	}
	return []db.CountUnreadArticlesByFeedForUserRow{}, nil
}
func (m *mockAFCtrl) MarkAsRead(_ context.Context, _ db.Querier, _, _ string) (bool, error) {
	return false, nil
}

var _ controllers.ArticleFeedControllerInterface = (*mockAFCtrl)(nil)

// mockOutboundCtrl is a no-op: no outbound links, so the read-time swap leaves bodies unchanged.
type mockOutboundCtrl struct{}

func (m *mockOutboundCtrl) Create(_ context.Context, _ db.Querier, _, _, _ string) (db.ArticleOutboundLink, error) {
	return db.ArticleOutboundLink{}, nil
}
func (m *mockOutboundCtrl) ListByArticleIDs(_ context.Context, _ db.Querier, _ []string) ([]db.ArticleOutboundLink, error) {
	return []db.ArticleOutboundLink{}, nil
}
func (m *mockOutboundCtrl) Retarget(_ context.Context, _ db.Querier, _, _ string) error { return nil }
func (m *mockOutboundCtrl) DeleteByArticleID(_ context.Context, _ db.Querier, _ string) error {
	return nil
}

var _ controllers.ArticleOutboundLinkControllerInterface = (*mockOutboundCtrl)(nil)

func buildFeedArticlesApp(feedCtrl controllers.FeedControllerInterface, afCtrl controllers.ArticleFeedControllerInterface) *fiber.App {
	app := fiber.New()
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	f := app.Group("/v1/feeds")
	testutils.AddRoute(f, fiber.MethodGet, "/:id/articles", append(authMiddleware, feedendpoints.FeedArticles(feedCtrl, afCtrl, &mockOutboundCtrl{}, fakeTxRunner, "")))
	return app
}

func afRow(id, title string, createdAt time.Time, isRead bool) db.ListArticlesByFeedForUserRow {
	return db.ListArticlesByFeedForUserRow{
		ID:              id,
		Status:          true,
		Title:           title,
		Content:         "content",
		UrlOriginal:     "https://example.com/" + id,
		Keywords:        json.RawMessage("[]"),
		SourceID:        "01900000-0000-7000-8000-000000000010",
		CreatedAt:       utctime.New(createdAt),
		IsRead:          isRead,
		SourceName:      "Example News",
		SourceStatus:    true,
		SourceUrl:       "https://source.example.com",
		SourceUrlRss:    "https://source.example.com/rss",
		SourceCreatedAt: utctime.New(createdAt),
	}
}

func boolPtr(v bool) *bool { return &v }

func getFeedArticles(t *testing.T, app *fiber.App, query string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/feed-1/articles"+query, nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestUnit_FeedArticles_FeedNotFound_404(t *testing.T) {
	requireNotProduction(t)

	feedCtrl := &mockFeedCtrl{findByIDFn: func(_ context.Context, _ db.Querier, _, _ string) (db.Feed, error) {
		return db.Feed{}, controllers.ErrFeedNotFound
	}}
	app := buildFeedArticlesApp(feedCtrl, &mockAFCtrl{})

	resp := getFeedArticles(t, app, "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUnit_FeedArticles_BadIsRead_400(t *testing.T) {
	requireNotProduction(t)

	app := buildFeedArticlesApp(&mockFeedCtrl{}, &mockAFCtrl{})
	resp := getFeedArticles(t, app, "?is_read=maybe")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUnit_FeedArticles_BadPeriod_400(t *testing.T) {
	requireNotProduction(t)

	app := buildFeedArticlesApp(&mockFeedCtrl{}, &mockAFCtrl{})
	resp := getFeedArticles(t, app, "?period_starting_at=not-a-date")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// The rows are narrowed in SQL, so what the handler owes is an accurate filter: is_read must arrive
// as a real *bool. The false case is the one that matters — a filter modelled as a plain bool would
// make "?is_read=false" indistinguishable from "no filter" and silently return read articles too.
func TestUnit_FeedArticles_ForwardsIsReadFilter(t *testing.T) {
	requireNotProduction(t)

	tests := []struct {
		name  string
		query string
		want  *bool
	}{
		{name: "absent means no filter", query: "", want: nil},
		{name: "true filters read", query: "?is_read=true", want: boolPtr(true)},
		{name: "false filters unread", query: "?is_read=false", want: boolPtr(false)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got controllers.ListFeedArticlesFilter
			afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string, filter controllers.ListFeedArticlesFilter) ([]db.ListArticlesByFeedForUserRow, int64, error) {
				got = filter
				return []db.ListArticlesByFeedForUserRow{}, 0, nil
			}}
			app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

			resp := getFeedArticles(t, app, tt.query)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.want, got.IsRead)
		})
	}
}

// The period bounds must reach the controller parsed and normalized to UTC — dates are UTC across
// the application, so a bound sent in another offset has to be converted, never passed through raw.
func TestUnit_FeedArticles_ForwardsPeriodFilter(t *testing.T) {
	requireNotProduction(t)

	var got controllers.ListFeedArticlesFilter
	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string, filter controllers.ListFeedArticlesFilter) ([]db.ListArticlesByFeedForUserRow, int64, error) {
		got = filter
		return []db.ListArticlesByFeedForUserRow{}, 0, nil
	}}
	app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

	t.Run("absent means no bounds", func(t *testing.T) {
		got = controllers.ListFeedArticlesFilter{}
		require.Equal(t, http.StatusOK, getFeedArticles(t, app, "").StatusCode)
		assert.Nil(t, got.PeriodStartingAt)
		assert.Nil(t, got.PeriodEndingAt)
	})

	t.Run("both bounds forwarded", func(t *testing.T) {
		got = controllers.ListFeedArticlesFilter{}
		resp := getFeedArticles(t, app, "?period_starting_at=2026-06-15T00:00:00Z&period_ending_at=2026-07-15T00:00:00Z")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotNil(t, got.PeriodStartingAt)
		require.NotNil(t, got.PeriodEndingAt)
		assert.Equal(t, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), *got.PeriodStartingAt)
		assert.Equal(t, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), *got.PeriodEndingAt)
	})

	t.Run("non-UTC offset is normalized to UTC", func(t *testing.T) {
		got = controllers.ListFeedArticlesFilter{}
		// 2026-06-15T00:00:00-03:00 is 2026-06-15T03:00:00Z.
		resp := getFeedArticles(t, app, "?period_starting_at=2026-06-15T00:00:00-03:00")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotNil(t, got.PeriodStartingAt)
		assert.Equal(t, time.Date(2026, 6, 15, 3, 0, 0, 0, time.UTC), got.PeriodStartingAt.UTC())
	})
}

func TestUnit_FeedArticles_EnvelopeAndIsRead(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string, _ controllers.ListFeedArticlesFilter) ([]db.ListArticlesByFeedForUserRow, int64, error) {
		return []db.ListArticlesByFeedForUserRow{afRow("a-1", "One", time.Now().UTC(), false)}, 1, nil
	}}
	app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

	resp := getFeedArticles(t, app, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var env struct {
		Docs       []map[string]any `json:"docs"`
		Pagination map[string]any   `json:"pagination"`
	}
	require.NoError(t, readJSON(resp, &env))
	require.Len(t, env.Docs, 1)
	assert.Equal(t, false, env.Docs[0]["is_read"])
	assert.Equal(t, float64(1), env.Pagination["total_count"])
}

func TestUnit_FeedArticles_WithSourcesTrue_PopulatesSource(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string, _ controllers.ListFeedArticlesFilter) ([]db.ListArticlesByFeedForUserRow, int64, error) {
		return []db.ListArticlesByFeedForUserRow{afRow("a-1", "One", time.Now().UTC(), false)}, 1, nil
	}}
	app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

	docs := decodePage(t, getFeedArticles(t, app, "?with_sources=true"))
	require.Len(t, docs, 1)
	source, ok := docs[0]["source"].(map[string]any)
	require.True(t, ok, "expected source to be populated")
	assert.Equal(t, "01900000-0000-7000-8000-000000000010", source["id"])
	assert.Equal(t, "Example News", source["name"])
	assert.Equal(t, "https://source.example.com", source["url"])
}

// Per conventions.md, with_{related_table_name} only populates on the literal value "true"; any
// other value (including absent) is silently ignored — never an error.
func TestUnit_FeedArticles_WithSourcesNotTrue_OmitsSource(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string, _ controllers.ListFeedArticlesFilter) ([]db.ListArticlesByFeedForUserRow, int64, error) {
		return []db.ListArticlesByFeedForUserRow{afRow("a-1", "One", time.Now().UTC(), false)}, 1, nil
	}}
	app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

	absentDocs := decodePage(t, getFeedArticles(t, app, ""))
	require.Len(t, absentDocs, 1)
	assert.Nil(t, absentDocs[0]["source"])

	garbageResp := getFeedArticles(t, app, "?with_sources=nope")
	assert.Equal(t, http.StatusOK, garbageResp.StatusCode)
	garbageDocs := decodePage(t, garbageResp)
	require.Len(t, garbageDocs, 1)
	assert.Nil(t, garbageDocs[0]["source"])
}

func TestUnit_FeedArticles_ControllerError_500(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string, _ controllers.ListFeedArticlesFilter) ([]db.ListArticlesByFeedForUserRow, int64, error) {
		return nil, 0, assertErr{}
	}}
	app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

	resp := getFeedArticles(t, app, "")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
