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
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
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
	generateAccessTokenFn  func(user db.User, refreshTokenID string) (string, error)
}

func (m *mockAuthCtrl) HandleGoogleCallback(ctx context.Context, code string) (schemas.AuthResponse, error) {
	return m.handleGoogleCallbackFn(ctx, code)
}

func (m *mockAuthCtrl) RefreshAccessToken(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error) {
	return m.refreshAccessTokenFn(ctx, refreshTokenID)
}

func (m *mockAuthCtrl) GenerateAccessToken(user db.User, refreshTokenID string) (string, error) {
	if m.generateAccessTokenFn != nil {
		return m.generateAccessTokenFn(user, refreshTokenID)
	}
	return "", nil
}

// mockRefreshTokenCtrl implements RefreshTokenControllerInterface for unit tests.
type mockRefreshTokenCtrl struct {
	findByIDFn  func(ctx context.Context, id string) (db.RefreshToken, error)
	revokeFn    func(ctx context.Context, id string) error
	revokeAllFn func(ctx context.Context, userID string) error
}

func (m *mockRefreshTokenCtrl) Create(ctx context.Context, userID string) (db.RefreshToken, error) {
	return db.RefreshToken{}, nil
}

func (m *mockRefreshTokenCtrl) FindByID(ctx context.Context, id string) (db.RefreshToken, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return fixtures.NewTestRefreshToken("any"), nil
}

func (m *mockRefreshTokenCtrl) Extend(ctx context.Context, id string) error { return nil }

func (m *mockRefreshTokenCtrl) Revoke(ctx context.Context, id string) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, id)
	}
	return nil
}

func (m *mockRefreshTokenCtrl) RevokeAll(ctx context.Context, userID string) error {
	if m.revokeAllFn != nil {
		return m.revokeAllFn(ctx, userID)
	}
	return nil
}

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
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), rtCtrl)

	auth := app.Group("/v1/auth")
	auth.Get("/google", authendpoints.GoogleLogin(testOAuth2Config()))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	auth.Post("/logout", append(authMiddleware, authendpoints.Logout(rtCtrl))...)
	auth.Delete("/invalidate", authendpoints.Invalidate(rtCtrl))
	auth.Delete("/invalidate-all", authendpoints.InvalidateAll(rtCtrl))

	return app
}
