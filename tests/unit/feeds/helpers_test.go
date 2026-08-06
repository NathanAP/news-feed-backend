package feeds_test

import (
	"context"
	"encoding/json"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
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

type mockFeedCtrl struct {
	createFn         func(ctx context.Context, q db.Querier, userID, name string, keywords []string) (db.Feed, error)
	findByIDFn       func(ctx context.Context, q db.Querier, id, userID string) (db.Feed, error)
	findCandidatesFn func(ctx context.Context, q db.Querier, keywords []string, inactiveDays int) ([]controllers.FeedCandidate, error)
	listFn           func(ctx context.Context, q db.Querier, userID string, filter controllers.ListFeedsFilter) ([]db.Feed, int64, error)
	listAllFn        func(ctx context.Context, q db.Querier, userID string) ([]db.Feed, error)
	updateFn         func(ctx context.Context, q db.Querier, id, userID, name string, keywords []string) (db.Feed, error)
	softDeleteFn     func(ctx context.Context, q db.Querier, id, userID string) error
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
func (m *mockFeedCtrl) FindCandidatesByKeywords(ctx context.Context, q db.Querier, keywords []string, inactiveDays int) ([]controllers.FeedCandidate, error) {
	if m.findCandidatesFn != nil {
		return m.findCandidatesFn(ctx, q, keywords, inactiveDays)
	}
	return []controllers.FeedCandidate{}, nil
}
func (m *mockFeedCtrl) List(ctx context.Context, q db.Querier, userID string, filter controllers.ListFeedsFilter) ([]db.Feed, int64, error) {
	if m.listFn != nil {
		return m.listFn(ctx, q, userID, filter)
	}
	feeds := []db.Feed{fixtures.NewTestFeed(userID)}
	return feeds, int64(len(feeds)), nil
}
func (m *mockFeedCtrl) ListAll(ctx context.Context, q db.Querier, userID string) ([]db.Feed, error) {
	if m.listAllFn != nil {
		return m.listAllFn(ctx, q, userID)
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
	app := fiber.New()
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)

	f := app.Group("/v1/feeds")
	testutils.AddRoute(f, fiber.MethodPost, "/create", append(authMiddleware, feedendpoints.CreateFeed(ctrl, fakeTxRunner)))
	testutils.AddRoute(f, fiber.MethodGet, "/:id", append(authMiddleware, feedendpoints.GetFeed(ctrl, fakeTxRunner)))
	testutils.AddRoute(f, fiber.MethodGet, "", append(authMiddleware, feedendpoints.ListFeeds(ctrl, fakeTxRunner)))
	testutils.AddRoute(f, fiber.MethodPut, "/:id", append(authMiddleware, feedendpoints.UpdateFeed(ctrl, fakeTxRunner)))
	testutils.AddRoute(f, fiber.MethodDelete, "/:id", append(authMiddleware, feedendpoints.DeleteFeed(ctrl, fakeTxRunner)))

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
