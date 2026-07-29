package sources_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
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

// mockRefreshTokenCtrl satisfies RefreshTokenControllerInterface for auth middleware.
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

// mockSourceCtrl holds optional function overrides for each method.
type mockSourceCtrl struct {
	createFn     func(ctx context.Context, q db.Querier, name, url, urlRss string) (db.Source, error)
	findByIDFn   func(ctx context.Context, q db.Querier, id string) (db.Source, error)
	listFn       func(ctx context.Context, q db.Querier, filter controllers.ListSourcesFilter) ([]db.Source, int64, error)
	listAllFn    func(ctx context.Context, q db.Querier) ([]db.Source, error)
	updateFn     func(ctx context.Context, q db.Querier, id, name, url, urlRss string) (db.Source, error)
	softDeleteFn func(ctx context.Context, q db.Querier, id string) error
}

func (m *mockSourceCtrl) Create(ctx context.Context, q db.Querier, name, url, urlRss string) (db.Source, error) {
	if m.createFn != nil {
		return m.createFn(ctx, q, name, url, urlRss)
	}
	return fixtures.NewTestSource(), nil
}
func (m *mockSourceCtrl) FindByID(ctx context.Context, q db.Querier, id string) (db.Source, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, q, id)
	}
	return fixtures.NewTestSource(), nil
}
func (m *mockSourceCtrl) List(ctx context.Context, q db.Querier, filter controllers.ListSourcesFilter) ([]db.Source, int64, error) {
	if m.listFn != nil {
		return m.listFn(ctx, q, filter)
	}
	sources := []db.Source{fixtures.NewTestSource()}
	return sources, int64(len(sources)), nil
}
func (m *mockSourceCtrl) ListAll(ctx context.Context, q db.Querier) ([]db.Source, error) {
	if m.listAllFn != nil {
		return m.listAllFn(ctx, q)
	}
	return []db.Source{fixtures.NewTestSource()}, nil
}
func (m *mockSourceCtrl) Update(ctx context.Context, q db.Querier, id, name, url, urlRss string) (db.Source, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, q, id, name, url, urlRss)
	}
	return fixtures.NewTestSource(), nil
}
func (m *mockSourceCtrl) SoftDelete(ctx context.Context, q db.Querier, id string) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, q, id)
	}
	return nil
}

var _ controllers.SourceControllerInterface = (*mockSourceCtrl)(nil)

// mockArticleCtrl satisfies ArticleControllerInterface for the article-discovery endpoint. By
// default FindByURLOriginal reports "not found", so no item is deduplicated.
type mockArticleCtrl struct {
	findByURLOriginalFn func(ctx context.Context, q db.Querier, urlOriginal string) (db.Article, error)
}

func (m *mockArticleCtrl) Create(_ context.Context, _ db.Querier, _, _, _, _ string, _ []string, _ *string) (db.Article, error) {
	return db.Article{}, nil
}
func (m *mockArticleCtrl) FindByID(_ context.Context, _ db.Querier, _ string) (db.Article, error) {
	return db.Article{}, controllers.ErrArticleNotFound
}
func (m *mockArticleCtrl) FindByURLOriginal(ctx context.Context, q db.Querier, urlOriginal string) (db.Article, error) {
	if m.findByURLOriginalFn != nil {
		return m.findByURLOriginalFn(ctx, q, urlOriginal)
	}
	return db.Article{}, controllers.ErrArticleNotFound
}
func (m *mockArticleCtrl) List(_ context.Context, _ db.Querier, _ controllers.ListArticlesFilter) ([]db.Article, int64, error) {
	return []db.Article{}, 0, nil
}
func (m *mockArticleCtrl) ListAll(_ context.Context, _ db.Querier) ([]db.Article, error) {
	return []db.Article{}, nil
}
func (m *mockArticleCtrl) SuggestKeywords(_ context.Context, _ db.Querier, _ []string, _ time.Time, _ int32) ([]schemas.KeywordSuggestion, string, error) {
	return []schemas.KeywordSuggestion{}, schemas.KeywordStrategyPopular, nil
}
func (m *mockArticleCtrl) Update(_ context.Context, _ db.Querier, _, _, _, _ string, _ []string, _ *string) (db.Article, error) {
	return db.Article{}, nil
}
func (m *mockArticleCtrl) SoftDelete(_ context.Context, _ db.Querier, _ string) error {
	return nil
}

var _ controllers.ArticleControllerInterface = (*mockArticleCtrl)(nil)

func buildApp(ctrl controllers.SourceControllerInterface, httpClient *http.Client) *fiber.App {
	return buildAppWithArticles(ctrl, &mockArticleCtrl{}, httpClient)
}

func buildAppWithArticles(ctrl controllers.SourceControllerInterface, articleCtrl controllers.ArticleControllerInterface, httpClient *http.Client) *fiber.App {
	return buildAppAs(ctrl, articleCtrl, httpClient, true)
}

// buildAppAsRegularUser wires the same routes for a caller whose database row is not an
// administrator. Only the admin-only routes behave differently, and that difference is the point:
// the token is identical in both apps, so a 403 here can only come from the database lookup.
func buildAppAsRegularUser(ctrl controllers.SourceControllerInterface) *fiber.App {
	return buildAppAs(ctrl, &mockArticleCtrl{}, http.DefaultClient, false)
}

func buildAppAs(ctrl controllers.SourceControllerInterface, articleCtrl controllers.ArticleControllerInterface, httpClient *http.Client, admin bool) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	requireAdmin := middlewares.NewRequireAdminMiddleware(middlewares.NewAdminResolver(
		[]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, &jwtmock.MockUserController{Admin: admin}, fakeTxRunner,
	))

	// Mirrors main.go: reading sources is open to every authenticated user, changing them is not.
	s := app.Group("/v1/sources")
	s.Post("/create", adminChain(authMiddleware, requireAdmin, sourceendpoints.CreateSource(ctrl, fakeTxRunner))...)
	s.Get("/rss-discovery", append(authMiddleware, sourceendpoints.RSSDiscovery(httpClient))...)
	s.Get("/:id/article-discovery", adminChain(authMiddleware, requireAdmin, sourceendpoints.SourceArticleDiscovery(ctrl, articleCtrl, fakeTxRunner, httpClient))...)
	s.Get("/:id", append(authMiddleware, sourceendpoints.GetSource(ctrl, fakeTxRunner))...)
	s.Get("", append(authMiddleware, sourceendpoints.ListSources(ctrl, fakeTxRunner))...)
	s.Put("/:id", adminChain(authMiddleware, requireAdmin, sourceendpoints.UpdateSource(ctrl, fakeTxRunner))...)
	s.Delete("/:id", adminChain(authMiddleware, requireAdmin, sourceendpoints.DeleteSource(ctrl, fakeTxRunner))...)

	return app
}

// adminChain copies the shared auth chain before appending, for the same reason main.go does: reusing
// one slice across registrations would let each route overwrite the previous route's handler.
func adminChain(authMiddleware []fiber.Handler, requireAdmin, handler fiber.Handler) []fiber.Handler {
	chain := make([]fiber.Handler, 0, len(authMiddleware)+2)
	chain = append(chain, authMiddleware...)
	return append(chain, requireAdmin, handler)
}

func defaultApp() *fiber.App {
	return buildApp(&mockSourceCtrl{}, http.DefaultClient)
}

func authHeader(t *testing.T) string {
	t.Helper()
	user := fixtures.NewTestUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)
	return "Bearer " + token
}

// adminClaimHeader builds a token that *claims* administrator. Whether the request is actually
// authorized still depends on the user row the app was built with, which is the point of having it.
func adminClaimHeader(t *testing.T) string {
	t.Helper()
	user := fixtures.NewTestAdminUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)
	return "Bearer " + token
}
