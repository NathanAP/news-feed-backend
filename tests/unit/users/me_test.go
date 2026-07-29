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
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
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

// fakeTxRunner is the unit-test substitute for the real WithTransaction: it runs the
// function without any database. Mock controllers ignore the nil querier.
func fakeTxRunner(ctx context.Context, fn func(q db.Querier) error) error {
	return fn(nil)
}

type mockRefreshTokenCtrl struct{}

func (m *mockRefreshTokenCtrl) Create(ctx context.Context, q db.Querier, userID string) (db.RefreshToken, error) {
	return db.RefreshToken{}, nil
}
func (m *mockRefreshTokenCtrl) FindByID(ctx context.Context, q db.Querier, id string) (db.RefreshToken, error) {
	return fixtures.NewTestRefreshToken("any"), nil
}
func (m *mockRefreshTokenCtrl) Extend(ctx context.Context, q db.Querier, id string) error { return nil }
func (m *mockRefreshTokenCtrl) Revoke(ctx context.Context, q db.Querier, id string) error { return nil }
func (m *mockRefreshTokenCtrl) RevokeAll(ctx context.Context, q db.Querier, userID string) error {
	return nil
}

var _ controllers.RefreshTokenControllerInterface = (*mockRefreshTokenCtrl)(nil)

func setupUsersApp() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)

	users := app.Group("/v1/users")
	users.Get("/me", append(authMiddleware, userendpoints.GetMe())...)

	return app
}

// mockPrefCtrl for unit tests of preferences endpoints
type mockPrefCtrl struct {
	updateFn func(ctx context.Context, userID string, params controllers.UpdatePreferencesParams) (db.UserPreference, error)
}

func (m *mockPrefCtrl) CreateDefault(ctx context.Context, q db.Querier, userID string) (db.UserPreference, error) {
	return db.UserPreference{}, nil
}
func (m *mockPrefCtrl) FindByUserID(ctx context.Context, q db.Querier, userID string) (db.UserPreference, error) {
	return fixtures.NewTestUserPreferences(userID), nil
}
func (m *mockPrefCtrl) Update(ctx context.Context, q db.Querier, userID string, params controllers.UpdatePreferencesParams) (db.UserPreference, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, userID, params)
	}
	return fixtures.NewTestUserPreferences(userID), nil
}
func (m *mockPrefCtrl) SoftDelete(ctx context.Context, q db.Querier, userID string) error { return nil }

var _ controllers.UserPreferencesControllerInterface = (*mockPrefCtrl)(nil)

// mockAuthForPrefs implements only what UpdatePreferences needs
type mockAuthForPrefs struct{}

func (m *mockAuthForPrefs) HandleGoogleCallback(ctx context.Context, code string) (schemas.AuthResponse, error) {
	return schemas.AuthResponse{}, nil
}
func (m *mockAuthForPrefs) RefreshAccessToken(ctx context.Context, id string) (schemas.AuthResponse, error) {
	return schemas.AuthResponse{}, nil
}
func (m *mockAuthForPrefs) GenerateAccessToken(user db.User, id string, prefs db.UserPreference) (string, error) {
	return "new-access-token", nil
}
func (m *mockAuthForPrefs) RegenerateFromClaims(ctx context.Context, q db.Querier, claims *schemas.Claims, prefs db.UserPreference) (string, error) {
	return "new-access-token", nil
}

var _ controllers.AuthControllerInterface = (*mockAuthForPrefs)(nil)

func setupPreferencesApp() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)

	users := app.Group("/v1/users")
	users.Get("/me/preferences", append(authMiddleware, userendpoints.GetPreferences())...)
	users.Put("/me/preferences", append(authMiddleware, userendpoints.UpdatePreferences(&mockPrefCtrl{}, &mockAuthForPrefs{}, fakeTxRunner, 3600))...)

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
	assert.False(t, body.Admin, "a regular user must not be reported as an administrator")
}

// TestGetMe_ReportsAdmin covers the field the client uses to decide whether to render the admin UI.
// The route echoes the caller's own claims, so what it reports is whatever the token was issued with.
func TestGetMe_ReportsAdmin(t *testing.T) {
	requireNotProduction(t)

	app := setupUsersApp()

	admin := fixtures.NewTestAdminUser()
	refreshToken := fixtures.NewTestRefreshToken(admin.ID)
	token, err := jwtmock.GenerateTestAccessToken(admin, refreshToken.ID)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body schemas.UserResponse
	require.NoError(t, readJSON(resp, &body))
	assert.True(t, body.Admin)
}

// The field must always be present, never omitted: a client cannot tell "regular user" from "this
// API version does not know about administrators" if the key disappears when false.
func TestGetMe_AlwaysIncludesAdminField(t *testing.T) {
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

	var raw map[string]any
	require.NoError(t, readJSON(resp, &raw))
	value, present := raw["admin"]
	assert.True(t, present, "admin must always be serialized")
	assert.Equal(t, false, value)
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
