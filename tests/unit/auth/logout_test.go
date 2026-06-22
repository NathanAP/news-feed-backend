package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	rtCtrl := &mockRefreshTokenCtrl{
		findByIDFn: func(ctx context.Context, id string) (db.RefreshToken, error) {
			return db.RefreshToken{}, errors.New("session revoked")
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
