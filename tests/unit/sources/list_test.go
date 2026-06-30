package sources_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

func TestListSources_Success(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier) ([]db.Source, error) {
			return []db.Source{fixtures.NewTestSource(), fixtures.NewTestSourceAlt()}, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Len(t, result, 2)
}

func TestListSources_Empty(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier) ([]db.Source, error) {
			return []db.Source{}, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Empty(t, result)
}

func TestListSources_FilterByURL(t *testing.T) {
	requireNotProduction(t)

	s1 := fixtures.NewTestSource()    // url: https://example.com
	s2 := fixtures.NewTestSourceAlt() // url: https://other-source.com
	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier) ([]db.Source, error) {
			return []db.Source{s1, s2}, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources?url=other-source", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Len(t, result, 1)
	assert.Equal(t, s2.Url, result[0]["url"])
}

func TestListSources_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier) ([]db.Source, error) {
			return nil, assert.AnError
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestListSources_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
