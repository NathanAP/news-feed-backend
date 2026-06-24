package users_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	userendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/users"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	_ "modernc.org/sqlite"
)

func readJSONPrefs(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func setupPreferencesIntegrationApp(t *testing.T) (*fiber.App, db.Querier, *controllers.UserPreferencesController, *controllers.AuthController) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)

	prefCtrl := controllers.NewUserPreferencesController(queries)
	userCtrl := controllers.NewUserController(queries, prefCtrl)
	refreshTokenCtrl := controllers.NewRefreshTokenController(queries, 30*24*time.Hour)
	authCtrl := controllers.NewAuthController(nil, userCtrl, refreshTokenCtrl, prefCtrl, []byte(jwtmock.TestJWTSecret), time.Hour)

	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	users := app.Group("/v1/users")
	users.Get("/me/preferences", append(authMiddleware, userendpoints.GetPreferences())...)
	users.Put("/me/preferences", append(authMiddleware, userendpoints.UpdatePreferences(prefCtrl, authCtrl, 3600))...)

	return app, queries, prefCtrl, authCtrl
}

func seedUserWithPrefs(t *testing.T, queries db.Querier, prefCtrl *controllers.UserPreferencesController, authCtrl *controllers.AuthController) (accessToken string) {
	t.Helper()

	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(context.Background(), db.CreateUserParams{
		ID:       user.ID,
		GoogleID: user.GoogleID,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	})
	require.NoError(t, err)

	refreshToken := fixtures.NewTestRefreshToken(user.ID)
	_, err = queries.CreateRefreshToken(context.Background(), db.CreateRefreshTokenParams{
		ID:        refreshToken.ID,
		UserID:    refreshToken.UserID,
		ExpiresAt: refreshToken.ExpiresAt,
	})
	require.NoError(t, err)

	prefs, err := prefCtrl.CreateDefault(context.Background(), user.ID)
	require.NoError(t, err)

	token, err := authCtrl.GenerateAccessToken(user, refreshToken.ID, prefs)
	require.NoError(t, err)
	return token
}

func TestGetPreferences_Integration_ReturnsCurrentPrefs(t *testing.T) {
	requireNotProduction(t)

	app, queries, prefCtrl, authCtrl := setupPreferencesIntegrationApp(t)
	token := seedUserWithPrefs(t, queries, prefCtrl, authCtrl)

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me/preferences", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, readJSONPrefs(resp, &body))
	assert.Equal(t, "dark", body["theme"])
	assert.Equal(t, "pt", body["language"])
	assert.Equal(t, true, body["translate_content"])
	assert.Equal(t, "mixed", body["ai_personality"])
}

func TestGetPreferences_Integration_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _, _, _ := setupPreferencesIntegrationApp(t)
	req, err := http.NewRequest(http.MethodGet, "/v1/users/me/preferences", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUpdatePreferences_Integration_PersistsAndReturnsNewToken(t *testing.T) {
	requireNotProduction(t)

	app, queries, prefCtrl, authCtrl := setupPreferencesIntegrationApp(t)
	token := seedUserWithPrefs(t, queries, prefCtrl, authCtrl)

	body := `{"theme":"light","language":"en","translate_content":false,"ai_personality":"fun"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, readJSONPrefs(resp, &result))
	assert.NotEmpty(t, result["access_token"])

	prefs, ok := result["preferences"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "light", prefs["theme"])
	assert.Equal(t, "en", prefs["language"])
	assert.Equal(t, false, prefs["translate_content"])
	assert.Equal(t, "fun", prefs["ai_personality"])
}

func TestUpdatePreferences_Integration_InvalidTheme(t *testing.T) {
	requireNotProduction(t)

	app, queries, prefCtrl, authCtrl := setupPreferencesIntegrationApp(t)
	token := seedUserWithPrefs(t, queries, prefCtrl, authCtrl)

	body := `{"theme":"invalid","language":"pt","translate_content":true,"ai_personality":"mixed"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
