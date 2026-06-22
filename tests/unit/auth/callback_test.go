package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/schemas"
)

func TestGoogleCallback_Success(t *testing.T) {
	requireNotProduction(t)

	authCtrl := &mockAuthCtrl{
		handleGoogleCallbackFn: func(ctx context.Context, code string) (schemas.AuthResponse, error) {
			return schemas.AuthResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				ExpiresIn:    3600,
			}, nil
		},
	}

	app := setupApp(authCtrl, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGoogleCallback_MissingCode(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGoogleCallback_AuthFailure(t *testing.T) {
	requireNotProduction(t)

	authCtrl := &mockAuthCtrl{
		handleGoogleCallbackFn: func(ctx context.Context, code string) (schemas.AuthResponse, error) {
			return schemas.AuthResponse{}, errors.New("oauth exchange failed")
		},
	}

	app := setupApp(authCtrl, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=bad-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
