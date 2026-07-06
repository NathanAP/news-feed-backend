package articles_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	articleendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/articles"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func fakeTxRunner(_ context.Context, fn func(q db.Querier) error) error {
	return fn(nil)
}

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// decodePage reads a paginated list response ({ docs, pagination }) and returns the docs slice.
func decodePage(t *testing.T, resp *http.Response) []map[string]any {
	t.Helper()
	var env struct {
		Docs []map[string]any `json:"docs"`
	}
	require.NoError(t, readJSON(resp, &env))
	return env.Docs
}

type mockRefreshTokenCtrl struct{}

func (m *mockRefreshTokenCtrl) Create(_ context.Context, _ db.Querier, _ string) (db.RefreshToken, error) {
	return db.RefreshToken{}, nil
}
func (m *mockRefreshTokenCtrl) FindByID(_ context.Context, _ db.Querier, _ string) (db.RefreshToken, error) {
	return fixtures.NewTestRefreshToken("any"), nil
}
func (m *mockRefreshTokenCtrl) Extend(_ context.Context, _ db.Querier, _ string) error { return nil }
func (m *mockRefreshTokenCtrl) Revoke(_ context.Context, _ db.Querier, _ string) error { return nil }
func (m *mockRefreshTokenCtrl) RevokeAll(_ context.Context, _ db.Querier, _ string) error {
	return nil
}

var _ controllers.RefreshTokenControllerInterface = (*mockRefreshTokenCtrl)(nil)

type mockArticleCtrl struct {
	createFn            func(ctx context.Context, q db.Querier, title, content, urlOriginal, sourceID string, keywords []string, languageOriginal *string) (db.Article, error)
	findByIDFn          func(ctx context.Context, q db.Querier, id string) (db.Article, error)
	findByURLOriginalFn func(ctx context.Context, q db.Querier, urlOriginal string) (db.Article, error)
	listFn              func(ctx context.Context, q db.Querier) ([]db.Article, error)
	updateFn            func(ctx context.Context, q db.Querier, id, title, content, urlOriginal string, keywords []string, languageOriginal *string) (db.Article, error)
	softDeleteFn        func(ctx context.Context, q db.Querier, id string) error
}

func (m *mockArticleCtrl) Create(ctx context.Context, q db.Querier, title, content, urlOriginal, sourceID string, keywords []string, languageOriginal *string) (db.Article, error) {
	if m.createFn != nil {
		return m.createFn(ctx, q, title, content, urlOriginal, sourceID, keywords, languageOriginal)
	}
	return fixtures.NewTestArticle(), nil
}
func (m *mockArticleCtrl) FindByID(ctx context.Context, q db.Querier, id string) (db.Article, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, q, id)
	}
	return fixtures.NewTestArticle(), nil
}
func (m *mockArticleCtrl) FindByURLOriginal(ctx context.Context, q db.Querier, urlOriginal string) (db.Article, error) {
	if m.findByURLOriginalFn != nil {
		return m.findByURLOriginalFn(ctx, q, urlOriginal)
	}
	return db.Article{}, controllers.ErrArticleNotFound
}
func (m *mockArticleCtrl) List(ctx context.Context, q db.Querier) ([]db.Article, error) {
	if m.listFn != nil {
		return m.listFn(ctx, q)
	}
	return []db.Article{fixtures.NewTestArticle()}, nil
}
func (m *mockArticleCtrl) Update(ctx context.Context, q db.Querier, id, title, content, urlOriginal string, keywords []string, languageOriginal *string) (db.Article, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, q, id, title, content, urlOriginal, keywords, languageOriginal)
	}
	return fixtures.NewTestArticle(), nil
}
func (m *mockArticleCtrl) SoftDelete(ctx context.Context, q db.Querier, id string) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, q, id)
	}
	return nil
}

var _ controllers.ArticleControllerInterface = (*mockArticleCtrl)(nil)

// mockArticleFeedCtrl is a no-op by default — returns empty records (nil is_read state).
type mockArticleFeedCtrl struct {
	createFn               func(ctx context.Context, q db.Querier, articleID, feedID string) (db.ArticleFeed, error)
	findByArticleAndUserFn func(ctx context.Context, q db.Querier, articleID, userID string) ([]db.ArticleFeed, error)
	listByFeedFn           func(ctx context.Context, q db.Querier, feedID, userID string) ([]db.ListArticlesByFeedForUserRow, error)
	markAsReadFn           func(ctx context.Context, q db.Querier, articleID, userID string) (bool, error)
}

func (m *mockArticleFeedCtrl) Create(ctx context.Context, q db.Querier, articleID, feedID string) (db.ArticleFeed, error) {
	if m.createFn != nil {
		return m.createFn(ctx, q, articleID, feedID)
	}
	return db.ArticleFeed{}, nil
}
func (m *mockArticleFeedCtrl) FindByArticleAndUser(ctx context.Context, q db.Querier, articleID, userID string) ([]db.ArticleFeed, error) {
	if m.findByArticleAndUserFn != nil {
		return m.findByArticleAndUserFn(ctx, q, articleID, userID)
	}
	return []db.ArticleFeed{}, nil
}
func (m *mockArticleFeedCtrl) ListArticlesByFeedForUser(ctx context.Context, q db.Querier, feedID, userID string) ([]db.ListArticlesByFeedForUserRow, error) {
	if m.listByFeedFn != nil {
		return m.listByFeedFn(ctx, q, feedID, userID)
	}
	return []db.ListArticlesByFeedForUserRow{}, nil
}
func (m *mockArticleFeedCtrl) MarkAsRead(ctx context.Context, q db.Querier, articleID, userID string) (bool, error) {
	if m.markAsReadFn != nil {
		return m.markAsReadFn(ctx, q, articleID, userID)
	}
	return false, nil
}

var _ controllers.ArticleFeedControllerInterface = (*mockArticleFeedCtrl)(nil)

func buildApp(ctrl controllers.ArticleControllerInterface, afCtrl controllers.ArticleFeedControllerInterface) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)

	a := app.Group("/v1/articles")
	a.Post("/create", append(authMiddleware, articleendpoints.CreateArticle(ctrl, fakeTxRunner))...)
	a.Put("/:id/read", append(authMiddleware, articleendpoints.MarkAsRead(ctrl, afCtrl, fakeTxRunner))...)
	a.Get("/:id", append(authMiddleware, articleendpoints.GetArticle(ctrl, afCtrl, fakeTxRunner))...)
	a.Get("", append(authMiddleware, articleendpoints.ListArticles(ctrl, fakeTxRunner))...)
	a.Put("/:id", append(authMiddleware, articleendpoints.UpdateArticle(ctrl, fakeTxRunner))...)
	a.Delete("/:id", append(authMiddleware, articleendpoints.DeleteArticle(ctrl, fakeTxRunner))...)

	return app
}

func defaultApp() *fiber.App {
	return buildApp(&mockArticleCtrl{}, &mockArticleFeedCtrl{})
}

func authHeader(t *testing.T) string {
	t.Helper()
	user := fixtures.NewTestUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)
	return "Bearer " + token
}

// validCreateBody returns a JSON payload satisfying all article validations.
func validCreateBody() string {
	return `{
		"title": "Test Article",
		"content": "# Test\n\nContent here.",
		"url_original": "https://example.com/news/test",
		"keywords": ["metallica","rock","metal","music","concert"],
		"source_id": "01900000-0000-7000-8000-000000000010",
		"language_original": "pt"
	}`
}
