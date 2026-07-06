package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

// fakeTxRunner is the unit-test substitute for the real WithTransaction: it runs the
// function without any database. Mock controllers ignore the nil querier.
func fakeTxRunner(ctx context.Context, fn func(q db.Querier) error) error {
	return fn(nil)
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

// mockAuthCtrl implements AuthControllerInterface for unit tests.
type mockAuthCtrl struct {
	handleGoogleCallbackFn func(ctx context.Context, code string) (schemas.AuthResponse, error)
	refreshAccessTokenFn   func(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error)
	generateAccessTokenFn  func(user db.User, refreshTokenID string, prefs db.UserPreference) (string, error)
	regenerateFromClaimsFn func(ctx context.Context, q db.Querier, claims *schemas.Claims, updatedPrefs db.UserPreference) (string, error)
}

func (m *mockAuthCtrl) HandleGoogleCallback(ctx context.Context, code string) (schemas.AuthResponse, error) {
	return m.handleGoogleCallbackFn(ctx, code)
}

func (m *mockAuthCtrl) RefreshAccessToken(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error) {
	return m.refreshAccessTokenFn(ctx, refreshTokenID)
}

func (m *mockAuthCtrl) GenerateAccessToken(user db.User, refreshTokenID string, prefs db.UserPreference) (string, error) {
	if m.generateAccessTokenFn != nil {
		return m.generateAccessTokenFn(user, refreshTokenID, prefs)
	}
	return "", nil
}

func (m *mockAuthCtrl) RegenerateFromClaims(ctx context.Context, q db.Querier, claims *schemas.Claims, updatedPrefs db.UserPreference) (string, error) {
	if m.regenerateFromClaimsFn != nil {
		return m.regenerateFromClaimsFn(ctx, q, claims, updatedPrefs)
	}
	return "new-access-token", nil
}

var _ controllers.AuthControllerInterface = (*mockAuthCtrl)(nil)

// mockRefreshTokenCtrl implements RefreshTokenControllerInterface for unit tests.
type mockRefreshTokenCtrl struct {
	findByIDFn  func(ctx context.Context, id string) (db.RefreshToken, error)
	revokeFn    func(ctx context.Context, id string) error
	revokeAllFn func(ctx context.Context, userID string) error
}

func (m *mockRefreshTokenCtrl) Create(ctx context.Context, q db.Querier, userID string) (db.RefreshToken, error) {
	return db.RefreshToken{}, nil
}

func (m *mockRefreshTokenCtrl) FindByID(ctx context.Context, q db.Querier, id string) (db.RefreshToken, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return fixtures.NewTestRefreshToken("any"), nil
}

func (m *mockRefreshTokenCtrl) Extend(ctx context.Context, q db.Querier, id string) error { return nil }

func (m *mockRefreshTokenCtrl) Revoke(ctx context.Context, q db.Querier, id string) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, id)
	}
	return nil
}

func (m *mockRefreshTokenCtrl) RevokeAll(ctx context.Context, q db.Querier, userID string) error {
	if m.revokeAllFn != nil {
		return m.revokeAllFn(ctx, userID)
	}
	return nil
}

var _ controllers.RefreshTokenControllerInterface = (*mockRefreshTokenCtrl)(nil)

// validRefreshTokenCtrl returns a mock that always passes session validation.
func validRefreshTokenCtrl() *mockRefreshTokenCtrl {
	return &mockRefreshTokenCtrl{}
}

// buildAuthRequest builds an HTTP request with a valid Bearer token.
func buildAuthRequest(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	user := fixtures.NewTestUser()
	refreshToken := fixtures.NewTestRefreshToken(user.ID)
	token, err := jwtmock.GenerateTestAccessToken(user, refreshToken.ID)
	require.NoError(t, err)

	var req *http.Request
	if body != nil {
		req, err = http.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, path, nil)
	}
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// setupApp builds a Fiber app with all auth routes wired for testing.
func setupApp(authCtrl *mockAuthCtrl, rtCtrl *mockRefreshTokenCtrl) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), rtCtrl, fakeTxRunner)

	auth := app.Group("/v1/auth")
	auth.Get("/google", authendpoints.GoogleLogin(testOAuth2Config(), []byte(jwtmock.TestJWTSecret), []string{testutils.TestOAuthRedirectURI}))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl, []byte(jwtmock.TestJWTSecret)))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	auth.Post("/logout", append(authMiddleware, authendpoints.Logout(rtCtrl, fakeTxRunner))...)
	auth.Delete("/invalidate", authendpoints.Invalidate(rtCtrl, fakeTxRunner))
	auth.Delete("/invalidate-all", authendpoints.InvalidateAll(rtCtrl, fakeTxRunner))

	return app
}
