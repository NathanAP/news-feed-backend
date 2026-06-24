package auth_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	db "github.com/nathanap/news-feed-backend/sqlc"
	_ "modernc.org/sqlite"
)

const testRefreshTokenExpiry = 30 * 24 * time.Hour

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func setupIntegrationApp(t *testing.T, oauth external.MockGoogleOAuth) (*fiber.App, *controllers.AuthController, *controllers.RefreshTokenController) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)

	prefCtrl := controllers.NewUserPreferencesController(queries)
	userCtrl := controllers.NewUserController(queries, prefCtrl)
	refreshTokenCtrl := controllers.NewRefreshTokenController(queries, testRefreshTokenExpiry)
	authCtrl := controllers.NewAuthController(&oauth, userCtrl, refreshTokenCtrl, prefCtrl, []byte(jwtmock.TestJWTSecret), time.Hour)

	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	auth := app.Group("/v1/auth")
	auth.Get("/google", authendpoints.GoogleLogin(testOAuth2Config()))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	auth.Post("/logout", append(authMiddleware, authendpoints.Logout(refreshTokenCtrl))...)
	auth.Delete("/invalidate", authendpoints.Invalidate(refreshTokenCtrl))
	auth.Delete("/invalidate-all", authendpoints.InvalidateAll(refreshTokenCtrl))

	return app, authCtrl, refreshTokenCtrl
}

func testOAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/v1/auth/google/callback",
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func base64Decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// extractUserIDFromToken decodes the JWT payload to extract the user_id claim.
func extractUserIDFromToken(t *testing.T, tokenString string) string {
	t.Helper()
	parts := strings.Split(tokenString, ".")
	require.Len(t, parts, 3, "expected JWT with 3 parts")

	payload, err := base64Decode(parts[1])
	require.NoError(t, err)

	var claims map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &claims))

	userID, ok := claims["user_id"].(string)
	require.True(t, ok && userID != "", "user_id not found in token claims")
	return userID
}

func loginAndGetTokens(t *testing.T, app *fiber.App) (accessToken, refreshToken string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, readJSON(resp, &body))

	access, ok := body["access_token"].(string)
	require.True(t, ok && access != "", "access_token missing from response")

	refresh, ok := body["refresh_token"].(string)
	require.True(t, ok && refresh != "", "refresh_token missing from response")

	return access, refresh
}
