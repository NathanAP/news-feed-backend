package auth_test

import (
	"context"
	"errors"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

// The session guard runs on every authenticated request, so how it classifies a failure decides
// what the whole client base sees when something breaks. A missing/expired session is the user's
// problem (401, "log in again"); anything else is ours and must be a 500. Conflating the two would
// tell every user with a perfectly valid session to log back in the moment the database blinks -
// and the login they attempt next would fail too, since the database is the thing that is down.

func TestValidateSession_TokenNotFound_401(t *testing.T) {
	requireNotProduction(t)

	rtCtrl := &mockRefreshTokenCtrl{
		findByIDFn: func(ctx context.Context, id string) (db.RefreshToken, error) {
			return db.RefreshToken{}, controllers.ErrRefreshTokenNotFound
		},
	}

	app := setupApp(&mockAuthCtrl{}, rtCtrl)
	resp, err := app.Test(buildAuthRequest(t, http.MethodPost, "/v1/auth/logout", nil))
	require.NoError(t, err)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestValidateSession_TokenExpired_401(t *testing.T) {
	requireNotProduction(t)

	rtCtrl := &mockRefreshTokenCtrl{
		findByIDFn: func(ctx context.Context, id string) (db.RefreshToken, error) {
			return db.RefreshToken{}, controllers.ErrRefreshTokenExpired
		},
	}

	app := setupApp(&mockAuthCtrl{}, rtCtrl)
	resp, err := app.Test(buildAuthRequest(t, http.MethodPost, "/v1/auth/logout", nil))
	require.NoError(t, err)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// The regression this pins: before 0.37.3.0 every error from the lookup became a 401, so an
// infrastructure failure was reported to the client as an expired session.
func TestValidateSession_DatabaseError_500_NotSignedOut(t *testing.T) {
	requireNotProduction(t)

	rtCtrl := &mockRefreshTokenCtrl{
		findByIDFn: func(ctx context.Context, id string) (db.RefreshToken, error) {
			return db.RefreshToken{}, errors.New("failed to find refresh token: connection refused")
		},
	}

	app := setupApp(&mockAuthCtrl{}, rtCtrl)
	resp, err := app.Test(buildAuthRequest(t, http.MethodPost, "/v1/auth/logout", nil))
	require.NoError(t, err)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode,
		"a database failure must not be reported to the client as an expired session")
}

// Fiber does not install recover by default: without it a panic in any handler unwinds past Fiber
// and takes the process down with it, which conventions.md forbids. main.go mounts it first; this
// pins that a panic stays a single failed request and the app keeps serving.
func TestRecoverMiddleware_PanicBecomes500_AppStaysUp(t *testing.T) {
	requireNotProduction(t)

	app := fiber.New()
	app.Use(recover.New())

	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), validRefreshTokenCtrl(), fakeTxRunner)
	testutils.AddRoute(app, fiber.MethodGet, "/v1/boom", append(authMiddleware, func(c fiber.Ctx) error {
		panic("unexpected failure inside a handler")
	}))
	app.Get("/v1/healthy", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(buildAuthRequest(t, http.MethodGet, "/v1/boom", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// The process survived the panic: a later request is still served normally.
	resp, err = app.Test(buildAuthRequest(t, http.MethodGet, "/v1/healthy", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
