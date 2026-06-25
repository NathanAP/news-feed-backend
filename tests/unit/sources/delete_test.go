package sources_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

func TestDeleteSource_Success(t *testing.T) {
	requireNotProduction(t)

	source := fixtures.NewTestSource()
	app := defaultApp()

	req, err := http.NewRequest(http.MethodDelete, "/v1/sources/"+source.ID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestDeleteSource_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Source, error) {
			return db.Source{}, controllers.ErrSourceNotFound
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodDelete, "/v1/sources/non-existent", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteSource_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Source, error) {
			return fixtures.NewTestSource(), nil
		},
		softDeleteFn: func(_ context.Context, _ db.Querier, _ string) error {
			return assert.AnError
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodDelete, "/v1/sources/some-id", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestDeleteSource_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, err := http.NewRequest(http.MethodDelete, "/v1/sources/some-id", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
