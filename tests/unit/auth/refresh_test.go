package auth_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
)

func TestRefreshToken_Success(t *testing.T) {
	requireNotProduction(t)

	authCtrl := &mockAuthCtrl{
		refreshAccessTokenFn: func(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error) {
			return schemas.AuthResponse{
				AccessToken:  "new-access-token",
				RefreshToken: refreshTokenID,
				ExpiresIn:    3600,
			}, nil
		},
	}

	app := setupApp(authCtrl, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"valid-refresh-token-id"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRefreshToken_MissingBody(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRefreshToken_ExpiredToken(t *testing.T) {
	requireNotProduction(t)

	authCtrl := &mockAuthCtrl{
		refreshAccessTokenFn: func(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error) {
			return schemas.AuthResponse{}, controllers.ErrRefreshTokenExpired
		},
	}

	app := setupApp(authCtrl, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"expired-token-id"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRefreshToken_NotFound(t *testing.T) {
	requireNotProduction(t)

	authCtrl := &mockAuthCtrl{
		refreshAccessTokenFn: func(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error) {
			return schemas.AuthResponse{}, controllers.ErrRefreshTokenNotFound
		},
	}

	app := setupApp(authCtrl, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"unknown-token-id"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRefreshToken_InternalError(t *testing.T) {
	requireNotProduction(t)

	authCtrl := &mockAuthCtrl{
		refreshAccessTokenFn: func(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error) {
			return schemas.AuthResponse{}, errors.New("unexpected error")
		},
	}

	app := setupApp(authCtrl, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"some-token-id"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
