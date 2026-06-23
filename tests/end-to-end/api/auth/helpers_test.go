package auth_test

import (
	"context"
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
	userendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/users"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	db "github.com/nathanap/news-feed-backend/sqlc"
	_ "modernc.org/sqlite"
)

const e2eRefreshExpiry = 30 * 24 * time.Hour

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

// setupE2EApp creates a full Fiber app with real DB and real controllers.
// Only OAuth2 is mocked, as required by E2E conventions.
func setupE2EApp(t *testing.T, oauth external.MockGoogleOAuth) (*fiber.App, db.Querier, *controllers.AuthController) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)

	userCtrl := controllers.NewUserController(queries)
	refreshTokenCtrl := controllers.NewRefreshTokenController(queries, e2eRefreshExpiry)
	authCtrl := controllers.NewAuthController(
		&oauth, userCtrl, refreshTokenCtrl,
		[]byte(jwtmock.TestJWTSecret), time.Hour,
	)

	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	auth := app.Group("/v1/auth")
	auth.Get("/google", authendpoints.GoogleLogin(testOAuth2Config()))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	auth.Post("/logout", append(authMiddleware, authendpoints.Logout(refreshTokenCtrl))...)
	auth.Delete("/invalidate", authendpoints.Invalidate(refreshTokenCtrl))
	auth.Delete("/invalidate-all", authendpoints.InvalidateAll(refreshTokenCtrl))

	users := app.Group("/v1/users")
	users.Get("/me", append(authMiddleware, userendpoints.GetMe())...)

	return app, queries, authCtrl
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

// seedSession creates a user + refresh_token directly in the DB and returns a valid access_token.
// This is the E2E mock approach for OAuth2: bypass the Google flow entirely.
func seedSession(t *testing.T, queries db.Querier, authCtrl *controllers.AuthController, googleID, email, name string) (accessToken, refreshTokenID string) {
	t.Helper()

	userCtrl := controllers.NewUserController(queries)
	refreshTokenCtrl := controllers.NewRefreshTokenController(queries, e2eRefreshExpiry)

	user, err := userCtrl.CreateUser(context.Background(), googleID, email, name, "")
	require.NoError(t, err)

	rt, err := refreshTokenCtrl.Create(context.Background(), user.ID)
	require.NoError(t, err)

	token, err := authCtrl.GenerateAccessToken(user, rt.ID)
	require.NoError(t, err)

	return token, rt.ID
}

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func extractUserIDFromToken(t *testing.T, tokenString string) string {
	t.Helper()
	parts := strings.Split(tokenString, ".")
	require.Len(t, parts, 3)

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var claims map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &claims))

	userID, ok := claims["user_id"].(string)
	require.True(t, ok && userID != "")
	return userID
}
