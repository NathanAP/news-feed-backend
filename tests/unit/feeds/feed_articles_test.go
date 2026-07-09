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
	"github.com/nathanap/news-feed-backend/services/controllers"
	feedendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/feeds"
	db "github.com/nathanap/news-feed-backend/sqlc"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

// mockAFCtrl is a minimal ArticleFeedControllerInterface for the feed-articles handler: only the
// list method is exercised here; the rest return zero values.
type mockAFCtrl struct {
	listByFeedFn  func(ctx context.Context, q db.Querier, feedID, userID string) ([]db.ListArticlesByFeedForUserRow, error)
	countUnreadFn func(ctx context.Context, q db.Querier, userID string) ([]db.CountUnreadArticlesByFeedForUserRow, error)
}

func (m *mockAFCtrl) Create(_ context.Context, _ db.Querier, _, _ string) (db.ArticlesFeed, error) {
	return db.ArticlesFeed{}, nil
}
func (m *mockAFCtrl) FindByArticleAndUser(_ context.Context, _ db.Querier, _, _ string) ([]db.ArticlesFeed, error) {
	return []db.ArticlesFeed{}, nil
}
func (m *mockAFCtrl) ListArticlesByFeedForUser(ctx context.Context, q db.Querier, feedID, userID string) ([]db.ListArticlesByFeedForUserRow, error) {
	if m.listByFeedFn != nil {
		return m.listByFeedFn(ctx, q, feedID, userID)
	}
	return []db.ListArticlesByFeedForUserRow{}, nil
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

func buildFeedArticlesApp(feedCtrl controllers.FeedControllerInterface, afCtrl controllers.ArticleFeedControllerInterface) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	f := app.Group("/v1/feeds")
	f.Get("/:id/articles", append(authMiddleware, feedendpoints.FeedArticles(feedCtrl, afCtrl, fakeTxRunner))...)
	return app
}

func afRow(id, title string, createdAt time.Time, isRead int64) db.ListArticlesByFeedForUserRow {
	return db.ListArticlesByFeedForUserRow{
		ID:              id,
		Status:          1,
		Title:           title,
		Content:         "content",
		UrlOriginal:     "https://example.com/" + id,
		Keywords:        "[]",
		SourceID:        "01900000-0000-7000-8000-000000000010",
		CreatedAt:       createdAt,
		IsRead:          isRead,
		SourceName:      "Example News",
		SourceStatus:    1,
		SourceUrl:       "https://source.example.com",
		SourceUrlRss:    "https://source.example.com/rss",
		SourceCreatedAt: createdAt,
	}
}

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

func TestUnit_FeedArticles_FiltersByIsRead(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ListArticlesByFeedForUserRow, error) {
		return []db.ListArticlesByFeedForUserRow{
			afRow("a-read", "Read one", time.Now().UTC(), 1),
			afRow("a-unread", "Unread one", time.Now().UTC(), 0),
		}, nil
	}}
	app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

	readDocs := decodePage(t, getFeedArticles(t, app, "?is_read=true"))
	require.Len(t, readDocs, 1)
	assert.Equal(t, "a-read", readDocs[0]["id"])
	assert.Equal(t, true, readDocs[0]["is_read"])

	unreadDocs := decodePage(t, getFeedArticles(t, app, "?is_read=false"))
	require.Len(t, unreadDocs, 1)
	assert.Equal(t, "a-unread", unreadDocs[0]["id"])
	assert.Equal(t, false, unreadDocs[0]["is_read"])
}

func TestUnit_FeedArticles_FiltersByPeriod(t *testing.T) {
	requireNotProduction(t)

	jun := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	jul := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ListArticlesByFeedForUserRow, error) {
		return []db.ListArticlesByFeedForUserRow{
			afRow("jul", "July", jul, 0),
			afRow("jun", "June", jun, 0),
		}, nil
	}}
	app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

	// From mid-June onwards: only July.
	fromDocs := decodePage(t, getFeedArticles(t, app, "?period_starting_at=2026-06-15T00:00:00Z"))
	require.Len(t, fromDocs, 1)
	assert.Equal(t, "jul", fromDocs[0]["id"])

	// Up to mid-June: only June.
	toDocs := decodePage(t, getFeedArticles(t, app, "?period_ending_at=2026-06-15T00:00:00Z"))
	require.Len(t, toDocs, 1)
	assert.Equal(t, "jun", toDocs[0]["id"])
}

func TestUnit_FeedArticles_EnvelopeAndIsRead(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ListArticlesByFeedForUserRow, error) {
		return []db.ListArticlesByFeedForUserRow{afRow("a-1", "One", time.Now().UTC(), 0)}, nil
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

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ListArticlesByFeedForUserRow, error) {
		return []db.ListArticlesByFeedForUserRow{afRow("a-1", "One", time.Now().UTC(), 0)}, nil
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

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ListArticlesByFeedForUserRow, error) {
		return []db.ListArticlesByFeedForUserRow{afRow("a-1", "One", time.Now().UTC(), 0)}, nil
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

	afCtrl := &mockAFCtrl{listByFeedFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ListArticlesByFeedForUserRow, error) {
		return nil, assertErr{}
	}}
	app := buildFeedArticlesApp(&mockFeedCtrl{}, afCtrl)

	resp := getFeedArticles(t, app, "")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
