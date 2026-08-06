package articles_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Article writes are an administrator escape hatch (PROJECT.md): the pipeline is what normally
// creates articles. Reading them stays open to every authenticated user, and these tests pin both
// halves — a regular user who can still read must not be able to write.

type adminRouteCase struct {
	name   string
	method string
	target string
	body   string
}

func adminOnlyArticleRoutes() []adminRouteCase {
	return []adminRouteCase{
		{"create", http.MethodPost, "/v1/articles/create", validCreateBody()},
		{"update", http.MethodPut, "/v1/articles/01900000-0000-7000-8000-000000000020", validCreateBody()},
		{"delete", http.MethodDelete, "/v1/articles/01900000-0000-7000-8000-000000000020", ""},
	}
}

func doRequest(t *testing.T, app *fiber.App, c adminRouteCase, header string) *http.Response {
	t.Helper()
	var req *http.Request
	var err error
	if c.body != "" {
		req, err = http.NewRequest(c.method, c.target, strings.NewReader(c.body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(c.method, c.target, nil)
		require.NoError(t, err)
	}
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestArticlesAdminRoutes_RejectRegularUser(t *testing.T) {
	requireNotProduction(t)

	for _, c := range adminOnlyArticleRoutes() {
		t.Run(c.name, func(t *testing.T) {
			app := buildAppAsRegularUser(&mockArticleCtrl{}, &mockArticleFeedCtrl{})
			resp := doRequest(t, app, c, authHeader(t))
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		})
	}
}

func TestArticlesAdminRoutes_RejectAnonymous(t *testing.T) {
	requireNotProduction(t)

	for _, c := range adminOnlyArticleRoutes() {
		t.Run(c.name, func(t *testing.T) {
			resp := doRequest(t, defaultApp(), c, "")
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestArticlesAdminRoutes_AllowAdmin(t *testing.T) {
	requireNotProduction(t)

	for _, c := range adminOnlyArticleRoutes() {
		t.Run(c.name, func(t *testing.T) {
			resp := doRequest(t, defaultApp(), c, authHeader(t))
			assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
			assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
		})
	}
}

// TestArticlesReadRoutes_StayOpenToRegularUsers is the counterweight: the guard must be on the write
// routes only. If someone mounts it on the group instead, every reader loses access and this fails.
func TestArticlesReadRoutes_StayOpenToRegularUsers(t *testing.T) {
	requireNotProduction(t)

	app := buildAppAsRegularUser(&mockArticleCtrl{}, &mockArticleFeedCtrl{})

	list := doRequest(t, app, adminRouteCase{method: http.MethodGet, target: "/v1/articles"}, authHeader(t))
	assert.Equal(t, http.StatusOK, list.StatusCode)

	get := doRequest(t, app, adminRouteCase{method: http.MethodGet, target: "/v1/articles/01900000-0000-7000-8000-000000000020"}, authHeader(t))
	assert.Equal(t, http.StatusOK, get.StatusCode)

	read := doRequest(t, app, adminRouteCase{method: http.MethodPut, target: "/v1/articles/01900000-0000-7000-8000-000000000020/read"}, authHeader(t))
	assert.NotEqual(t, http.StatusForbidden, read.StatusCode)
}
