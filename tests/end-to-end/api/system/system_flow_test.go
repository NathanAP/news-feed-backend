package system_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
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
	_ "modernc.org/sqlite"
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

// setupE2EApp wires the real handlers exactly like main.go around the maintenance guard:
// /health and the toggle are exempt; auth and the sources group are guarded.
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

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	api := app.Group("/v1")

	api.Get("/health", healthendpoints.Check(systemCtrl, runTx))
	api.Put("/system/app-status", systemendpoints.UpdateAppStatus(systemCtrl, runTx))
	api.Use(middlewares.NewAppStatusMiddleware(systemCtrl, runTx))

	auth := api.Group("/auth")
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl, []byte(jwtmock.TestJWTSecret)))

	s := api.Group("/sources")
	s.Post("/create", append(authMiddleware, sourceendpoints.CreateSource(sourceCtrl, runTx))...)
	s.Get("", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx))...)

	return app, queries
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

// TestE2E_System_MaintenanceFlow walks the full maintenance lifecycle: healthy → login →
// business works → switch off → business blocked (503) → switch back on while off → business
// works again.
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

	// Login (app is on, auth is guarded but reachable).
	token := loginViaCallback(t, app, queries)

	// Business route works while on.
	create := do(t, app, http.MethodPost, "/v1/sources/create", token,
		`{"name":"E2E Sys News","url":"https://e2e-sys.com","url_rss":"https://e2e-sys.com/rss.xml"}`)
	assert.Equal(t, http.StatusCreated, create.StatusCode)

	// Switch the application off.
	off := do(t, app, http.MethodPut, "/v1/system/app-status", "", `{"app_status":false}`)
	assert.Equal(t, http.StatusOK, off.StatusCode)

	// Health still answers, now reporting maintenance.
	health2 := do(t, app, http.MethodGet, "/v1/health", "", "")
	assert.Equal(t, http.StatusOK, health2.StatusCode)
	var health2Body map[string]any
	require.NoError(t, readJSON(health2, &health2Body))
	assert.Equal(t, false, health2Body["app_status"])

	// Business routes are blocked with 503.
	blockedCreate := do(t, app, http.MethodPost, "/v1/sources/create", token,
		`{"name":"Blocked News","url":"https://blocked.com","url_rss":"https://blocked.com/rss.xml"}`)
	assert.Equal(t, http.StatusServiceUnavailable, blockedCreate.StatusCode)

	blockedList := do(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusServiceUnavailable, blockedList.StatusCode)

	// The toggle stays reachable while off — this is what prevents a permanent lockout.
	on := do(t, app, http.MethodPut, "/v1/system/app-status", "", `{"app_status":true}`)
	assert.Equal(t, http.StatusOK, on.StatusCode)

	// Business works again, and the source created earlier is still there.
	list := do(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusOK, list.StatusCode)
	listed := decodePage(t, list)
	assert.Len(t, listed, 1)
}

func TestE2E_System_ToggleMissingField(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupE2EApp(t, testOAuth())

	resp := do(t, app, http.MethodPut, "/v1/system/app-status", "", `{}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
