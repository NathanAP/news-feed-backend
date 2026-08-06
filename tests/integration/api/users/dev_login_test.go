package users_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	userendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/users"
	db "github.com/nathanap/news-feed-backend/sqlc"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

func setupDevLoginApp(t *testing.T) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	userCtrl := controllers.NewUserController()
	prefCtrl := controllers.NewUserPreferencesController()
	refreshCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authCtrl := controllers.NewAuthController(nil, userCtrl, refreshCtrl, prefCtrl, runTx, []byte(jwtmock.TestJWTSecret), time.Hour)

	app := fiber.New()
	app.Post("/v1/users/dev-login", userendpoints.DevLogin(userCtrl, prefCtrl, refreshCtrl, authCtrl, runTx, 3600))

	return app, queries
}

// seedDevUser inserts a user carrying the fixed dev google_id plus its default preferences.
func seedDevUser(t *testing.T, queries db.Querier) {
	t.Helper()
	userCtrl := controllers.NewUserController()
	// CreateUser also creates default preferences; runs on the shared single-connection pool.
	_, err := userCtrl.CreateUser(context.Background(), queries, schemas.DevUserGoogleID, "dev@test.local", "Dev", "")
	require.NoError(t, err)
}

func TestIntegration_DevLogin_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupDevLoginApp(t)
	seedDevUser(t, queries)

	req, _ := http.NewRequest(http.MethodPost, "/v1/users/dev-login", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSONPrefs(resp, &result))
	assert.NotEmpty(t, result["access_token"])
	assert.NotEmpty(t, result["refresh_token"])
	assert.Equal(t, float64(3600), result["expires_in"])
}

func TestIntegration_DevLogin_NoDevUser(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupDevLoginApp(t)

	req, _ := http.NewRequest(http.MethodPost, "/v1/users/dev-login", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
