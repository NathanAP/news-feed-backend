package auth_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	"github.com/nathanap/news-feed-backend/services/oauthstate"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
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
	runTx := controllers.NewTransactionRunner(database)

	prefCtrl := controllers.NewUserPreferencesController()
	userCtrl := controllers.NewUserController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(testRefreshTokenExpiry)
	authCtrl := controllers.NewAuthController(&oauth, userCtrl, refreshTokenCtrl, prefCtrl, runTx, []byte(jwtmock.TestJWTSecret), time.Hour)

	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New()
	auth := app.Group("/v1/auth")
	auth.Get("/google", authendpoints.GoogleLogin(testOAuth2Config(), []byte(jwtmock.TestJWTSecret), []string{testutils.TestOAuthRedirectURI}))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl, []byte(jwtmock.TestJWTSecret)))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	testutils.AddRoute(auth, fiber.MethodPost, "/logout", append(authMiddleware, authendpoints.Logout(refreshTokenCtrl, runTx)))
	auth.Delete("/invalidate", authendpoints.Invalidate(refreshTokenCtrl, runTx))
	auth.Delete("/invalidate-all", authendpoints.InvalidateAll(refreshTokenCtrl, runTx))

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
	return testutils.CompleteOAuthLogin(t, app, []byte(jwtmock.TestJWTSecret))
}

// validState mints a state token for the test redirect URI, as GET /auth/google would.
func validState(t *testing.T) string {
	t.Helper()
	state, err := oauthstate.Generate([]byte(jwtmock.TestJWTSecret), testutils.TestOAuthRedirectURI)
	require.NoError(t, err)
	return state
}
