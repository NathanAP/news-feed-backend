package system_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	healthendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/health"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
	systemendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/system"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// decodePage reads a paginated list response ({ docs, pagination }) and returns the docs slice.
func decodePage(t *testing.T, resp *http.Response) []map[string]any {
	t.Helper()
	var env struct {
		Docs []map[string]any `json:"docs"`
	}
	require.NoError(t, readJSON(resp, &env))
	return env.Docs
}

// setupE2EApp wires the real handlers exactly like main.go around the maintenance guard: /health,
// the toggle and the whole /auth group are exempt; the sources group is guarded.
func setupE2EApp(t *testing.T, oauth external.MockGoogleOAuth) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	prefCtrl := controllers.NewUserPreferencesController()
	userCtrl := controllers.NewUserController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authCtrl := controllers.NewAuthController(
		&oauth, userCtrl, refreshTokenCtrl, prefCtrl,
		runTx, []byte(jwtmock.TestJWTSecret), time.Hour,
	)
	systemCtrl := controllers.NewSystemController()
	sourceCtrl := controllers.NewSourceController()
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)
	adminResolver := middlewares.NewAdminResolver([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, userCtrl, runTx)
	requireAdmin := middlewares.NewRequireAdminMiddleware(adminResolver)

	app := fiber.New()
	api := app.Group("/v1")

	api.Get("/health", healthendpoints.Check(systemCtrl, runTx))
	testutils.AddRoute(api, fiber.MethodPut, "/system/app-status", adminChain(authMiddleware, requireAdmin, systemendpoints.UpdateAppStatus(systemCtrl, runTx)))

	// Exempt from the guard, like in main.go: authentication has to survive a maintenance window or
	// an administrator whose token expires during one can never get back in.
	auth := api.Group("/auth")
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl, []byte(jwtmock.TestJWTSecret)))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))

	api.Use(middlewares.NewAppStatusMiddleware(systemCtrl, runTx, adminResolver))

	s := api.Group("/sources")
	testutils.AddRoute(s, fiber.MethodPost, "/create", adminChain(authMiddleware, requireAdmin, sourceendpoints.CreateSource(sourceCtrl, runTx)))
	testutils.AddRoute(s, fiber.MethodGet, "", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx)))

	return app, queries
}

// adminChain copies the shared auth chain before appending, for the same reason main.go does: reusing
// one slice across registrations would let each route overwrite the previous route's handler.
func adminChain(authMiddleware []fiber.Handler, requireAdmin, handler fiber.Handler) []fiber.Handler {
	chain := make([]fiber.Handler, 0, len(authMiddleware)+2)
	chain = append(chain, authMiddleware...)
	return append(chain, requireAdmin, handler)
}

// promote / demote flip the administrator flag straight in the database, which is exactly how an
// administrator is made today (PROJECT.md: no endpoint for it yet, it is a manual change).
func promote(t *testing.T, queries db.Querier, token string) {
	t.Helper()
	setAdmin(t, queries, token, true)
}

func demote(t *testing.T, queries db.Querier, token string) {
	t.Helper()
	setAdmin(t, queries, token, false)
}

func setAdmin(t *testing.T, queries db.Querier, token string, admin bool) {
	t.Helper()
	claims := testutils.ParseTestClaims(t, token, []byte(jwtmock.TestJWTSecret))
	_, err := queries.SetUserAdmin(context.Background(), db.SetUserAdminParams{ID: claims.UserID, Admin: admin})
	require.NoError(t, err)
}

func loginViaCallback(t *testing.T, app *fiber.App, queries db.Querier) string {
	t.Helper()

	token, refreshTokenID := testutils.CompleteOAuthLogin(t, app, []byte(jwtmock.TestJWTSecret))

	// Sanity-check that the login persisted the session and user.
	rt, err := queries.FindRefreshTokenByID(context.Background(), refreshTokenID)
	require.NoError(t, err)
	_, err = queries.FindUserByID(context.Background(), rt.UserID)
	require.NoError(t, err)

	return token
}

func do(t *testing.T, app *fiber.App, method, target, token, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, target, strings.NewReader(body))
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func testOAuth() external.MockGoogleOAuth {
	return external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-system-1", Email: "system@example.com", Name: "System User",
		},
	}
}

// TestE2E_System_MaintenanceFlow walks the full lifecycle a real operator goes through: healthy →
// login as a regular user → refused an administrator action → promoted in the database → the same
// action works → switch the application off → regular users blocked while the administrator keeps
// working → demoted mid-maintenance and immediately blocked → promoted again and switch back on.
func TestE2E_System_MaintenanceFlow(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupE2EApp(t, testOAuth())

	// Healthy from the start.
	health := do(t, app, http.MethodGet, "/v1/health", "", "")
	assert.Equal(t, http.StatusOK, health.StatusCode)
	var healthBody map[string]any
	require.NoError(t, readJSON(health, &healthBody))
	assert.Equal(t, true, healthBody["app_status"])
	assert.NotEmpty(t, healthBody["server_time"])

	// Login. A user created by the login flow is never an administrator.
	token := loginViaCallback(t, app, queries)
	assert.False(t, testutils.ParseTestClaims(t, token, []byte(jwtmock.TestJWTSecret)).Admin,
		"a freshly registered user must not come back as an administrator")

	// Reading is open to them; creating a source is not.
	assert.Equal(t, http.StatusOK, do(t, app, http.MethodGet, "/v1/sources", token, "").StatusCode)
	refused := do(t, app, http.MethodPost, "/v1/sources/create", token,
		`{"name":"E2E Sys News","url":"https://e2e-sys.com","url_rss":"https://e2e-sys.com/rss.xml"}`)
	assert.Equal(t, http.StatusForbidden, refused.StatusCode)

	// Promotion is a database change (there is no endpoint for it yet). No re-login: authorization
	// reads the row, so the token they already hold starts working immediately.
	promote(t, queries, token)

	create := do(t, app, http.MethodPost, "/v1/sources/create", token,
		`{"name":"E2E Sys News","url":"https://e2e-sys.com","url_rss":"https://e2e-sys.com/rss.xml"}`)
	assert.Equal(t, http.StatusCreated, create.StatusCode)

	// Switch the application off.
	off := do(t, app, http.MethodPut, "/v1/system/app-status", token, `{"app_status":false}`)
	assert.Equal(t, http.StatusOK, off.StatusCode)

	// Health still answers, now reporting maintenance.
	health2 := do(t, app, http.MethodGet, "/v1/health", "", "")
	assert.Equal(t, http.StatusOK, health2.StatusCode)
	var health2Body map[string]any
	require.NoError(t, readJSON(health2, &health2Body))
	assert.Equal(t, false, health2Body["app_status"])

	// Anonymous and regular traffic is blocked with 503 ...
	assert.Equal(t, http.StatusServiceUnavailable, do(t, app, http.MethodGet, "/v1/sources", "", "").StatusCode)

	// ... while the administrator keeps working right through the maintenance window.
	stillWorking := do(t, app, http.MethodPost, "/v1/sources/create", token,
		`{"name":"E2E During Maintenance","url":"https://e2e-maint.com","url_rss":"https://e2e-maint.com/rss.xml"}`)
	assert.Equal(t, http.StatusCreated, stillWorking.StatusCode)

	// Demoting them takes effect on the very next request, token unchanged.
	demote(t, queries, token)
	assert.Equal(t, http.StatusServiceUnavailable, do(t, app, http.MethodGet, "/v1/sources", token, "").StatusCode)

	// Promote again and bring the application back up.
	promote(t, queries, token)
	on := do(t, app, http.MethodPut, "/v1/system/app-status", token, `{"app_status":true}`)
	assert.Equal(t, http.StatusOK, on.StatusCode)

	// Business works again, and both sources created along the way are there.
	list := do(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusOK, list.StatusCode)
	listed := decodePage(t, list)
	assert.Len(t, listed, 2)
}

// TestE2E_System_RefreshSurvivesMaintenance is the lockout test. An administrator's access token
// expires in about an hour; if /auth/refresh answered 503 during maintenance, they could never renew
// it and never reach the toggle that ends the maintenance.
func TestE2E_System_RefreshSurvivesMaintenance(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupE2EApp(t, testOAuth())

	accessToken, refreshTokenID := testutils.CompleteOAuthLogin(t, app, []byte(jwtmock.TestJWTSecret))
	promote(t, queries, accessToken)

	off := do(t, app, http.MethodPut, "/v1/system/app-status", accessToken, `{"app_status":false}`)
	require.Equal(t, http.StatusOK, off.StatusCode)

	refreshed := do(t, app, http.MethodPost, "/v1/auth/refresh", "", `{"refresh_token":"`+refreshTokenID+`"}`)
	require.Equal(t, http.StatusOK, refreshed.StatusCode)

	var body map[string]any
	require.NoError(t, readJSON(refreshed, &body))
	newToken, ok := body["access_token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, newToken)

	// The renewed token carries the promotion, and gets the administrator back to the toggle.
	assert.True(t, testutils.ParseTestClaims(t, newToken, []byte(jwtmock.TestJWTSecret)).Admin)
	on := do(t, app, http.MethodPut, "/v1/system/app-status", newToken, `{"app_status":true}`)
	assert.Equal(t, http.StatusOK, on.StatusCode)
}

func TestE2E_System_ToggleMissingField(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupE2EApp(t, testOAuth())
	token := loginViaCallback(t, app, queries)
	promote(t, queries, token)

	resp := do(t, app, http.MethodPut, "/v1/system/app-status", token, `{}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_System_ToggleRequiresAdmin(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupE2EApp(t, testOAuth())

	anonymous := do(t, app, http.MethodPut, "/v1/system/app-status", "", `{"app_status":false}`)
	assert.Equal(t, http.StatusUnauthorized, anonymous.StatusCode)

	token := loginViaCallback(t, app, queries)
	regular := do(t, app, http.MethodPut, "/v1/system/app-status", token, `{"app_status":false}`)
	assert.Equal(t, http.StatusForbidden, regular.StatusCode)

	// The application was never actually taken down by those attempts.
	health := do(t, app, http.MethodGet, "/v1/health", "", "")
	var body map[string]any
	require.NoError(t, readJSON(health, &body))
	assert.Equal(t, true, body["app_status"])
}
