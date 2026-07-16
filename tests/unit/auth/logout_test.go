package auth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

func TestLogout_Success(t *testing.T) {
	requireNotProduction(t)

	rtCtrl := validRefreshTokenCtrl()
	app := setupApp(&mockAuthCtrl{}, rtCtrl)

	req := buildAuthRequest(t, http.MethodPost, "/v1/auth/logout", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestLogout_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestLogout_InvalidToken(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer invalid.token.here")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestLogout_RevokedSession(t *testing.T) {
	requireNotProduction(t)

	// Revoking sets status = FALSE + removed_at, and FindRefreshTokenByID filters both out, so the
	// real controller answers a revoked session with the typed ErrRefreshTokenNotFound. The mock
	// must return that same typed error: since 0.37.3.0 the session guard tells a missing/expired
	// session (401) apart from an infrastructure failure (500), so an untyped error here would
	// exercise the 500 path and stop testing what this case is about.
	rtCtrl := &mockRefreshTokenCtrl{
		findByIDFn: func(ctx context.Context, id string) (db.RefreshToken, error) {
			return db.RefreshToken{}, controllers.ErrRefreshTokenNotFound
		},
	}

	app := setupApp(&mockAuthCtrl{}, rtCtrl)

	user := fixtures.NewTestUser()
	token, err := jwtmock.GenerateTestAccessToken(user, "revoked-token-id")
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
