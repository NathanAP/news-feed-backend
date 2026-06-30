package system_test

import (
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
	healthendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/health"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
	systemendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/system"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
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

// setupIntegrationApp mirrors main.go wiring around the maintenance guard: /health and the
// toggle are registered before the guard (exempt); a guarded business route (list sources)
// is registered after it.
func setupIntegrationApp(t *testing.T) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	systemCtrl := controllers.NewSystemController()
	sourceCtrl := controllers.NewSourceController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	api := app.Group("/v1")

	api.Get("/health", healthendpoints.Check(systemCtrl, runTx))
	api.Put("/system/app-status", systemendpoints.UpdateAppStatus(systemCtrl, runTx))
	api.Use(middlewares.NewAppStatusMiddleware(systemCtrl, runTx))

	s := api.Group("/sources")
	s.Get("", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx))...)

	return app, queries
}

func seedUser(t *testing.T, queries db.Querier) (db.User, string) {
	t.Helper()
	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID:       user.ID,
		GoogleID: user.GoogleID,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	})
	require.NoError(t, err)

	rt := fixtures.NewTestRefreshToken(user.ID)
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID:        rt.ID,
		UserID:    rt.UserID,
		ExpiresAt: rt.ExpiresAt,
	})
	require.NoError(t, err)

	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)
	return user, token
}

func getJSON(t *testing.T, app *fiber.App, method, target, token, body string) *http.Response {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req, err := http.NewRequest(method, target, reader)
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

// ── Health ─────────────────────────────────────────────────────────────────────

func TestIntegration_Health_ReportsControlSnapshot(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupIntegrationApp(t)

	resp := getJSON(t, app, http.MethodGet, "/v1/health", "", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, true, result["app_status"]) // seeded active by the migration
	assert.NotEmpty(t, result["server_time"])
	_, hasVersion := result["version"]
	assert.True(t, hasVersion)
}

func TestIntegration_Health_StaysReachableDuringMaintenance(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	resp := getJSON(t, app, http.MethodGet, "/v1/health", "", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, false, result["app_status"])
}

// ── Guard ──────────────────────────────────────────────────────────────────────

func TestIntegration_Guard_AllowsBusinessRouteWhenActive(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	_, token := seedUser(t, queries)

	resp := getJSON(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestIntegration_Guard_BlocksBeforeAuth proves the maintenance check runs ahead of auth: an
// unauthenticated request still gets 503 (not 401) while the app is off.
func TestIntegration_Guard_BlocksBeforeAuth(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	resp := getJSON(t, app, http.MethodGet, "/v1/sources", "", "")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestIntegration_Guard_BlocksAuthenticatedBusinessRoute(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	_, token := seedUser(t, queries)
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	resp := getJSON(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// ── Toggle ─────────────────────────────────────────────────────────────────────

func TestIntegration_Toggle_OffThenBackOn(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	_, token := seedUser(t, queries)

	// Turn the app off.
	off := getJSON(t, app, http.MethodPut, "/v1/system/app-status", "", `{"app_status":false}`)
	assert.Equal(t, http.StatusOK, off.StatusCode)
	var offBody map[string]any
	require.NoError(t, readJSON(off, &offBody))
	assert.Equal(t, false, offBody["app_status"])

	// Business route is now blocked.
	blocked := getJSON(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusServiceUnavailable, blocked.StatusCode)

	// The toggle itself stays reachable while off — this is what avoids a permanent lockout.
	on := getJSON(t, app, http.MethodPut, "/v1/system/app-status", "", `{"app_status":true}`)
	assert.Equal(t, http.StatusOK, on.StatusCode)
	var onBody map[string]any
	require.NoError(t, readJSON(on, &onBody))
	assert.Equal(t, true, onBody["app_status"])

	// Business route works again.
	restored := getJSON(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusOK, restored.StatusCode)
}

func TestIntegration_Toggle_MissingField(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupIntegrationApp(t)

	resp := getJSON(t, app, http.MethodPut, "/v1/system/app-status", "", `{}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
