package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
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
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
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
	runTx := controllers.NewTransactionRunner(database)

	prefCtrl := controllers.NewUserPreferencesController()
	userCtrl := controllers.NewUserController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(e2eRefreshExpiry)
	authCtrl := controllers.NewAuthController(
		&oauth, userCtrl, refreshTokenCtrl, prefCtrl,
		runTx, []byte(jwtmock.TestJWTSecret), time.Hour,
	)

	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	auth := app.Group("/v1/auth")
	auth.Get("/google", authendpoints.GoogleLogin(testOAuth2Config()))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	auth.Post("/logout", append(authMiddleware, authendpoints.Logout(refreshTokenCtrl, runTx))...)
	auth.Delete("/invalidate", authendpoints.Invalidate(refreshTokenCtrl, runTx))
	auth.Delete("/invalidate-all", authendpoints.InvalidateAll(refreshTokenCtrl, runTx))

	users := app.Group("/v1/users")
	users.Get("/me", append(authMiddleware, userendpoints.GetMe())...)
	users.Get("/me/preferences", append(authMiddleware, userendpoints.GetPreferences())...)
	users.Put("/me/preferences", append(authMiddleware, userendpoints.UpdatePreferences(prefCtrl, authCtrl, runTx, 3600))...)

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

// loginViaCallback performs a full Google OAuth2 login through the real callback endpoint
// using MockGoogleOAuth. Verifies that the user and refresh_token were persisted in the DB
// and returns both records along with the access_token. Use this fixture in tests that need
// an authenticated session created through the real login flow.
func loginViaCallback(t *testing.T, app *fiber.App, queries db.Querier) (user db.User, rt db.RefreshToken, accessToken string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=any-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "callback should return 200")

	var body map[string]interface{}
	require.NoError(t, readJSON(resp, &body))

	accessToken, _ = body["access_token"].(string)
	refreshTokenID, _ := body["refresh_token"].(string)
	require.NotEmpty(t, accessToken, "access_token missing from callback response")
	require.NotEmpty(t, refreshTokenID, "refresh_token missing from callback response")

	ctx := context.Background()

	rt, err = queries.FindRefreshTokenByID(ctx, refreshTokenID)
	require.NoError(t, err, "refresh_token should be persisted in DB after login")

	user, err = queries.FindUserByID(ctx, rt.UserID)
	require.NoError(t, err, "user should be persisted in DB after login")

	return user, rt, accessToken
}

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}
