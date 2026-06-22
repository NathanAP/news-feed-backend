package users_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	userendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/users"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	db "github.com/nathanap/news-feed-backend/sqlc"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

type mockRefreshTokenCtrl struct{}

func (m *mockRefreshTokenCtrl) Create(ctx context.Context, userID string) (db.RefreshToken, error) {
	return db.RefreshToken{}, nil
}
func (m *mockRefreshTokenCtrl) FindByID(ctx context.Context, id string) (db.RefreshToken, error) {
	return fixtures.NewTestRefreshToken("any"), nil
}
func (m *mockRefreshTokenCtrl) Extend(ctx context.Context, id string) error { return nil }
func (m *mockRefreshTokenCtrl) Revoke(ctx context.Context, id string) error  { return nil }
func (m *mockRefreshTokenCtrl) RevokeAll(ctx context.Context, userID string) error { return nil }

var _ controllers.RefreshTokenControllerInterface = (*mockRefreshTokenCtrl)(nil)

func setupUsersApp() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{})

	users := app.Group("/v1/users")
	users.Get("/me", append(authMiddleware, userendpoints.GetMe())...)

	return app
}

func TestGetMe_Success(t *testing.T) {
	requireNotProduction(t)

	app := setupUsersApp()

	user := fixtures.NewTestUser()
	refreshToken := fixtures.NewTestRefreshToken(user.ID)
	token, err := jwtmock.GenerateTestAccessToken(user, refreshToken.ID)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body schemas.UserResponse
	require.NoError(t, readJSON(resp, &body))
	assert.Equal(t, user.ID, body.ID)
	assert.Equal(t, user.Email, body.Email)
	assert.Equal(t, user.Name, body.Name)
}

func TestGetMe_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := setupUsersApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGetMe_InvalidToken(t *testing.T) {
	requireNotProduction(t)

	app := setupUsersApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
