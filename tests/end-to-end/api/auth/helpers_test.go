package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	userendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/users"
	"github.com/nathanap/news-feed-backend/services/oauthstate"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
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
	requireAdmin := middlewares.NewRequireAdminMiddleware(
		middlewares.NewAdminResolver([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, userCtrl, runTx),
	)

	app := fiber.New()

	auth := app.Group("/v1/auth")
	auth.Get("/google", authendpoints.GoogleLogin(testOAuth2Config(), []byte(jwtmock.TestJWTSecret), []string{testutils.TestOAuthRedirectURI}))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl, []byte(jwtmock.TestJWTSecret)))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	testutils.AddRoute(auth, fiber.MethodPost, "/logout", append(authMiddleware, authendpoints.Logout(refreshTokenCtrl, runTx)))
	// Administrator-only: these revoke *other people's* sessions.
	testutils.AddRoute(auth, fiber.MethodDelete, "/invalidate", adminChain(authMiddleware, requireAdmin, authendpoints.Invalidate(refreshTokenCtrl, runTx)))
	testutils.AddRoute(auth, fiber.MethodDelete, "/invalidate-all", adminChain(authMiddleware, requireAdmin, authendpoints.InvalidateAll(refreshTokenCtrl, runTx)))

	users := app.Group("/v1/users")
	testutils.AddRoute(users, fiber.MethodGet, "/me", append(authMiddleware, userendpoints.GetMe()))
	testutils.AddRoute(users, fiber.MethodGet, "/me/preferences", append(authMiddleware, userendpoints.GetPreferences()))
	testutils.AddRoute(users, fiber.MethodPut, "/me/preferences", append(authMiddleware, userendpoints.UpdatePreferences(prefCtrl, authCtrl, runTx, 3600)))

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

// adminChain copies the shared auth chain before appending, for the same reason main.go does: reusing
// one slice across registrations would let each route overwrite the previous route's handler.
func adminChain(authMiddleware []fiber.Handler, requireAdmin, handler fiber.Handler) []fiber.Handler {
	chain := make([]fiber.Handler, 0, len(authMiddleware)+2)
	chain = append(chain, authMiddleware...)
	return append(chain, requireAdmin, handler)
}

// testOAuth is the Google identity used by the flows that only need *some* logged-in user.
func testOAuth() external.MockGoogleOAuth {
	return external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-auth-admin", Email: "auth-admin@example.com", Name: "Auth Admin",
		},
	}
}

// promote flips the administrator flag on an existing user, the way it is actually done today
// (PROJECT.md: a manual database change, no endpoint for it yet).
func promote(t *testing.T, queries db.Querier, userID string) {
	t.Helper()
	_, err := queries.SetUserAdmin(context.Background(), db.SetUserAdminParams{ID: userID, Admin: true})
	require.NoError(t, err)
}

// loginViaCallback performs a full Google OAuth2 login through the real two-step flow (start →
// callback) using MockGoogleOAuth: GET /auth/google issues a signed state, then the callback
// consumes it and redirects back with the tokens in the fragment. It verifies that the user and
// refresh_token were persisted and returns both records along with the access_token.
func loginViaCallback(t *testing.T, app *fiber.App, queries db.Querier) (user db.User, rt db.RefreshToken, accessToken string) {
	t.Helper()

	// Step 1: start the login — the server issues a signed state inside the Google auth URL.
	startReq, err := http.NewRequest(http.MethodGet, "/v1/auth/google?redirect_uri="+url.QueryEscape(testutils.TestOAuthRedirectURI), nil)
	require.NoError(t, err)
	startResp, err := app.Test(startReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, startResp.StatusCode, "login should redirect to Google")
	state := extractStateFromGoogleURL(t, startResp.Header.Get("Location"))

	// Step 2: Google redirects to our callback with a code + the same state → client redirect + fragment.
	cbReq, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=any-code&state="+url.QueryEscape(state), nil)
	require.NoError(t, err)
	cbResp, err := app.Test(cbReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, cbResp.StatusCode, "callback should redirect to the client")

	var refreshTokenID string
	accessToken, refreshTokenID = testutils.ParseTokensFromRedirect(t, cbResp.Header.Get("Location"))

	ctx := context.Background()

	rt, err = queries.FindRefreshTokenByID(ctx, refreshTokenID)
	require.NoError(t, err, "refresh_token should be persisted in DB after login")

	user, err = queries.FindUserByID(ctx, rt.UserID)
	require.NoError(t, err, "user should be persisted in DB after login")

	return user, rt, accessToken
}

// validState mints a state token for the test redirect URI, as GET /auth/google would.
func validState(t *testing.T) string {
	t.Helper()
	state, err := oauthstate.Generate([]byte(jwtmock.TestJWTSecret), testutils.TestOAuthRedirectURI)
	require.NoError(t, err)
	return state
}

// extractStateFromGoogleURL pulls the state parameter out of the Google auth URL the login endpoint
// redirects to.
func extractStateFromGoogleURL(t *testing.T, location string) string {
	t.Helper()
	require.NotEmpty(t, location, "expected a redirect to Google")
	u, err := url.Parse(location)
	require.NoError(t, err)
	state := u.Query().Get("state")
	require.NotEmpty(t, state, "state missing from Google auth URL")
	return state
}

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}
