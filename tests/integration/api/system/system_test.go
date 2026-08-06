package system_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	healthendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/health"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
	systemendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/system"
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

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// setupIntegrationApp mirrors main.go wiring around the maintenance guard: /health, the toggle and
// the /auth group are registered before the guard (exempt); a guarded business route (list sources)
// is registered after it.
func setupIntegrationApp(t *testing.T) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	systemCtrl := controllers.NewSystemController()
	sourceCtrl := controllers.NewSourceController()
	userCtrl := controllers.NewUserController()
	prefCtrl := controllers.NewUserPreferencesController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authCtrl := controllers.NewAuthController(
		&oauth2.Config{}, userCtrl, refreshTokenCtrl, prefCtrl, runTx, []byte(jwtmock.TestJWTSecret), time.Hour,
	)

	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)
	adminResolver := middlewares.NewAdminResolver([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, userCtrl, runTx)
	requireAdmin := middlewares.NewRequireAdminMiddleware(adminResolver)

	app := fiber.New()
	api := app.Group("/v1")

	api.Get("/health", healthendpoints.Check(systemCtrl, runTx))

	toggleChain := make([]fiber.Handler, 0, len(authMiddleware)+2)
	toggleChain = append(toggleChain, authMiddleware...)
	toggleChain = append(toggleChain, requireAdmin, systemendpoints.UpdateAppStatus(systemCtrl, runTx))
	testutils.AddRoute(api, fiber.MethodPut, "/system/app-status", toggleChain)

	// Registered ahead of the guard on purpose: without this, an administrator whose access token
	// expires during maintenance can never refresh it, and therefore can never reach the toggle that
	// ends the maintenance.
	api.Post("/auth/refresh", authendpoints.RefreshToken(authCtrl))

	api.Use(middlewares.NewAppStatusMiddleware(systemCtrl, runTx, adminResolver))

	s := api.Group("/sources")
	testutils.AddRoute(s, fiber.MethodGet, "", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx)))

	return app, queries
}

func seedUser(t *testing.T, queries db.Querier) (db.User, string) {
	t.Helper()
	user, token, _ := seedUserFixture(t, queries, fixtures.NewTestUser())
	return user, token
}

func seedAdmin(t *testing.T, queries db.Querier) (db.User, string) {
	t.Helper()
	user, token, _ := seedUserFixture(t, queries, fixtures.NewTestAdminUser())
	return user, token
}

// seedUserFixture persists a user with its session and preferences, and returns a signed token for
// it. Ids for the session and preferences are generated instead of taken from the fixtures because
// several tests seed a regular user and an administrator into the same database, and the fixtures
// carry fixed ids that would collide on the second insert.
func seedUserFixture(t *testing.T, queries db.Querier, user db.User) (db.User, string, string) {
	t.Helper()
	created, err := fixtures.CreateUser(t.Context(), queries, user)
	require.NoError(t, err)

	prefID, err := uuid.NewV7()
	require.NoError(t, err)
	prefs := fixtures.NewTestUserPreferences(created.ID)
	_, err = queries.CreateUserPreferences(t.Context(), db.CreateUserPreferencesParams{
		ID:                  prefID.String(),
		UserID:              created.ID,
		LanguageToTranslate: prefs.LanguageToTranslate,
		AiPersonality:       prefs.AiPersonality,
	})
	require.NoError(t, err)

	rtID, err := uuid.NewV7()
	require.NoError(t, err)
	rt := fixtures.NewTestRefreshToken(created.ID)
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID:        rtID.String(),
		UserID:    rt.UserID,
		ExpiresAt: rt.ExpiresAt,
	})
	require.NoError(t, err)

	token, err := jwtmock.GenerateTestAccessToken(created, rtID.String())
	require.NoError(t, err)
	return created, token, rtID.String()
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
	_, adminToken := seedAdmin(t, queries)

	// Turn the app off.
	off := getJSON(t, app, http.MethodPut, "/v1/system/app-status", adminToken, `{"app_status":false}`)
	assert.Equal(t, http.StatusOK, off.StatusCode)
	var offBody map[string]any
	require.NoError(t, readJSON(off, &offBody))
	assert.Equal(t, false, offBody["app_status"])

	// Business route is now blocked.
	blocked := getJSON(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusServiceUnavailable, blocked.StatusCode)

	// The toggle itself stays reachable while off — this is what avoids a permanent lockout.
	on := getJSON(t, app, http.MethodPut, "/v1/system/app-status", adminToken, `{"app_status":true}`)
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

	app, queries := setupIntegrationApp(t)
	_, adminToken := seedAdmin(t, queries)

	resp := getJSON(t, app, http.MethodPut, "/v1/system/app-status", adminToken, `{}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Toggle_RequiresAdmin(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	_, token := seedUser(t, queries)

	anonymous := getJSON(t, app, http.MethodPut, "/v1/system/app-status", "", `{"app_status":false}`)
	assert.Equal(t, http.StatusUnauthorized, anonymous.StatusCode)

	regular := getJSON(t, app, http.MethodPut, "/v1/system/app-status", token, `{"app_status":false}`)
	assert.Equal(t, http.StatusForbidden, regular.StatusCode)

	// And the switch really was left untouched by the refused calls.
	health := getJSON(t, app, http.MethodGet, "/v1/health", "", "")
	var body map[string]any
	require.NoError(t, readJSON(health, &body))
	assert.Equal(t, true, body["app_status"])
}

// ── Administrator bypass of maintenance ────────────────────────────────────────

func TestIntegration_Maintenance_AdminKeepsWorking(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	_, token := seedUser(t, queries)
	_, adminToken := seedAdmin(t, queries)
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	blocked := getJSON(t, app, http.MethodGet, "/v1/sources", token, "")
	assert.Equal(t, http.StatusServiceUnavailable, blocked.StatusCode)

	allowed := getJSON(t, app, http.MethodGet, "/v1/sources", adminToken, "")
	assert.Equal(t, http.StatusOK, allowed.StatusCode)
}

// TestIntegration_Maintenance_DemotedAdminIsBlocked proves the bypass reads the database and not the
// token: the token still says admin, the row no longer does.
func TestIntegration_Maintenance_DemotedAdminIsBlocked(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	admin, adminToken := seedAdmin(t, queries)
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	allowed := getJSON(t, app, http.MethodGet, "/v1/sources", adminToken, "")
	require.Equal(t, http.StatusOK, allowed.StatusCode)

	_, err := queries.SetUserAdmin(t.Context(), db.SetUserAdminParams{ID: admin.ID, Admin: false})
	require.NoError(t, err)

	blocked := getJSON(t, app, http.MethodGet, "/v1/sources", adminToken, "")
	assert.Equal(t, http.StatusServiceUnavailable, blocked.StatusCode)
}

// TestIntegration_Maintenance_ForgedAdminClaimIsBlocked covers the same idea from the other side: a
// regular user carrying a token that claims administrator (only possible if the claim were ever
// trusted) gets no further than any other regular user.
func TestIntegration_Maintenance_ForgedAdminClaimIsBlocked(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	user, _, refreshTokenID := seedUserFixture(t, queries, fixtures.NewTestUser())
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	claimingAdmin := user
	claimingAdmin.Admin = true
	forged, err := jwtmock.GenerateTestAccessToken(claimingAdmin, refreshTokenID)
	require.NoError(t, err)

	resp := getJSON(t, app, http.MethodGet, "/v1/sources", forged, "")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// TestIntegration_Maintenance_RefreshStaysReachable is the deadlock test: /auth/refresh must answer
// while the application is off, otherwise an administrator whose access token expires mid-maintenance
// can never get a new one — and therefore can never reach the toggle to bring the API back.
func TestIntegration_Maintenance_RefreshStaysReachable(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	_, _, refreshTokenID := seedUserFixture(t, queries, fixtures.NewTestAdminUser())
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	resp := getJSON(t, app, http.MethodPost, "/v1/auth/refresh", "", `{"refresh_token":"`+refreshTokenID+`"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, readJSON(resp, &body))
	require.NotEmpty(t, body["access_token"])

	// And the freshly minted token really does get the administrator through the guard.
	allowed := getJSON(t, app, http.MethodGet, "/v1/sources", body["access_token"].(string), "")
	assert.Equal(t, http.StatusOK, allowed.StatusCode)
}

// A regular user refreshing during maintenance is fine — they get a token and still hit 503
// everywhere that matters. This is the accepted cost of keeping /auth reachable.
func TestIntegration_Maintenance_RegularUserRefreshesButStaysBlocked(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	_, _, refreshTokenID := seedUserFixture(t, queries, fixtures.NewTestUser())
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	resp := getJSON(t, app, http.MethodPost, "/v1/auth/refresh", "", `{"refresh_token":"`+refreshTokenID+`"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, readJSON(resp, &body))

	blocked := getJSON(t, app, http.MethodGet, "/v1/sources", body["access_token"].(string), "")
	assert.Equal(t, http.StatusServiceUnavailable, blocked.StatusCode)
}

// An administrator who logged out must not walk past maintenance on a token that merely has not
// expired yet: the bypass validates the session, not just the signature.
func TestIntegration_Maintenance_LoggedOutAdminIsBlocked(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	_, adminToken, refreshTokenID := seedUserFixture(t, queries, fixtures.NewTestAdminUser())
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	require.NoError(t, queries.RevokeRefreshToken(t.Context(), refreshTokenID))

	resp := getJSON(t, app, http.MethodGet, "/v1/sources", adminToken, "")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}
