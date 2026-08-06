package system_test

import (
	"context"
	"encoding/json"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	systemendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/system"
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

const testSystemID = "01900000-0000-7000-8000-000000000001"

// mockSystemCtrl holds optional function overrides for each method.
type mockSystemCtrl struct {
	getFn                      func(ctx context.Context, q db.Querier) (db.System, error)
	updateAppStatusFn          func(ctx context.Context, q db.Querier, active bool) (db.System, error)
	updateLastArticleDiscovery func(ctx context.Context, q db.Querier, at time.Time) (db.System, error)
}

func (m *mockSystemCtrl) Get(ctx context.Context, q db.Querier) (db.System, error) {
	if m.getFn != nil {
		return m.getFn(ctx, q)
	}
	return db.System{ID: testSystemID, AppStatus: true}, nil
}

func (m *mockSystemCtrl) UpdateAppStatus(ctx context.Context, q db.Querier, active bool) (db.System, error) {
	if m.updateAppStatusFn != nil {
		return m.updateAppStatusFn(ctx, q, active)
	}
	return db.System{ID: testSystemID, AppStatus: active}, nil
}

func (m *mockSystemCtrl) UpdateLastArticleDiscovery(ctx context.Context, q db.Querier, at time.Time) (db.System, error) {
	if m.updateLastArticleDiscovery != nil {
		return m.updateLastArticleDiscovery(ctx, q, at)
	}
	return db.System{ID: testSystemID, AppStatus: true}, nil
}

var _ controllers.SystemControllerInterface = (*mockSystemCtrl)(nil)

// mockRefreshTokenCtrl satisfies RefreshTokenControllerInterface so the auth middleware accepts any
// well-signed token; the session itself is not what these tests are about.
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

func buildApp(ctrl controllers.SystemControllerInterface) *fiber.App {
	return buildAppAs(ctrl, true)
}

func buildAppAsRegularUser(ctrl controllers.SystemControllerInterface) *fiber.App {
	return buildAppAs(ctrl, false)
}

// buildAppAs mirrors main.go: the maintenance toggle is administrator-only (0.40) and is registered
// ahead of the maintenance guard, so it stays reachable while the application is off.
func buildAppAs(ctrl controllers.SystemControllerInterface, admin bool) *fiber.App {
	app := fiber.New()
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	requireAdmin := middlewares.NewRequireAdminMiddleware(middlewares.NewAdminResolver(
		[]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, &jwtmock.MockUserController{Admin: admin}, fakeTxRunner,
	))

	chain := make([]fiber.Handler, 0, len(authMiddleware)+2)
	chain = append(chain, authMiddleware...)
	chain = append(chain, requireAdmin, systemendpoints.UpdateAppStatus(ctrl, fakeTxRunner))
	testutils.AddRoute(app, fiber.MethodPut, "/v1/system/app-status", chain)

	return app
}

func defaultApp() *fiber.App {
	return buildApp(&mockSystemCtrl{})
}

func authHeader(t *testing.T) string {
	t.Helper()
	user := fixtures.NewTestUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)
	return "Bearer " + token
}
