package feeds_test

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
	feedendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/feeds"
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

type mockFeedCtrl struct {
	createFn     func(ctx context.Context, q db.Querier, userID, name string, keywords []string) (db.Feed, error)
	findByIDFn   func(ctx context.Context, q db.Querier, id, userID string) (db.Feed, error)
	listFn       func(ctx context.Context, q db.Querier, userID string) ([]db.Feed, error)
	updateFn     func(ctx context.Context, q db.Querier, id, userID, name string, keywords []string) (db.Feed, error)
	softDeleteFn func(ctx context.Context, q db.Querier, id, userID string) error
}

func (m *mockFeedCtrl) Create(ctx context.Context, q db.Querier, userID, name string, keywords []string) (db.Feed, error) {
	if m.createFn != nil {
		return m.createFn(ctx, q, userID, name, keywords)
	}
	return fixtures.NewTestFeed(userID), nil
}
func (m *mockFeedCtrl) FindByID(ctx context.Context, q db.Querier, id, userID string) (db.Feed, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, q, id, userID)
	}
	return fixtures.NewTestFeed(userID), nil
}
func (m *mockFeedCtrl) List(ctx context.Context, q db.Querier, userID string) ([]db.Feed, error) {
	if m.listFn != nil {
		return m.listFn(ctx, q, userID)
	}
	return []db.Feed{fixtures.NewTestFeed(userID)}, nil
}
func (m *mockFeedCtrl) Update(ctx context.Context, q db.Querier, id, userID, name string, keywords []string) (db.Feed, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, q, id, userID, name, keywords)
	}
	return fixtures.NewTestFeed(userID), nil
}
func (m *mockFeedCtrl) SoftDelete(ctx context.Context, q db.Querier, id, userID string) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, q, id, userID)
	}
	return nil
}

var _ controllers.FeedControllerInterface = (*mockFeedCtrl)(nil)

func buildApp(ctrl controllers.FeedControllerInterface) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)

	f := app.Group("/v1/feeds")
	f.Post("/create", append(authMiddleware, feedendpoints.CreateFeed(ctrl, fakeTxRunner))...)
	f.Get("/:id", append(authMiddleware, feedendpoints.GetFeed(ctrl, fakeTxRunner))...)
	f.Get("", append(authMiddleware, feedendpoints.ListFeeds(ctrl, fakeTxRunner))...)
	f.Put("/:id", append(authMiddleware, feedendpoints.UpdateFeed(ctrl, fakeTxRunner))...)
	f.Delete("/:id", append(authMiddleware, feedendpoints.DeleteFeed(ctrl, fakeTxRunner))...)

	return app
}

func defaultApp() *fiber.App {
	return buildApp(&mockFeedCtrl{})
}

func authHeader(t *testing.T) string {
	t.Helper()
	user := fixtures.NewTestUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)
	return "Bearer " + token
}

func validCreateBody() string {
	return `{"name":"Metallica Feed","keywords":["metallica","rock","metal","music","concert"]}`
}
