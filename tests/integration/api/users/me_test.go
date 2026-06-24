package users_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	userendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/users"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	_ "modernc.org/sqlite"
)

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func setupUsersIntegrationApp(t *testing.T) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)

	prefCtrl := controllers.NewUserPreferencesController(queries)
	_ = prefCtrl
	refreshTokenCtrl := controllers.NewRefreshTokenController(queries, 30*24*time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	users := app.Group("/v1/users")
	users.Get("/me", append(authMiddleware, userendpoints.GetMe())...)

	return app, queries
}

func TestGetMe_Integration_ReturnsUserData(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupUsersIntegrationApp(t)

	// Seed a user and refresh token in the real DB
	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID:       user.ID,
		GoogleID: user.GoogleID,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	})
	require.NoError(t, err)

	refreshToken := fixtures.NewTestRefreshToken(user.ID)
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID:        refreshToken.ID,
		UserID:    refreshToken.UserID,
		ExpiresAt: refreshToken.ExpiresAt,
	})
	require.NoError(t, err)

	accessToken, err := jwtmock.GenerateTestAccessToken(user, refreshToken.ID)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body schemas.UserResponse
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, user.ID, body.ID)
	assert.Equal(t, user.Email, body.Email)
	assert.Equal(t, user.Name, body.Name)
}

func TestGetMe_Integration_RevokedSession(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupUsersIntegrationApp(t)

	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID:       user.ID,
		GoogleID: user.GoogleID,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	})
	require.NoError(t, err)

	// Generate token with a refresh_token_id that doesn't exist in DB
	accessToken, err := jwtmock.GenerateTestAccessToken(user, "non-existent-refresh-token-id")
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGetMe_Integration_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupUsersIntegrationApp(t)

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
