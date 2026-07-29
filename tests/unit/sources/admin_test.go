package sources_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

// The source write routes are administrator-only (PROJECT.md). These tests pin the three answers the
// guard can give — 401 without a session, 403 for a regular user, and through for an administrator —
// on every one of them, so a future route added to the group without the guard shows up here.

type adminRouteCase struct {
	name   string
	method string
	target string
	body   string
}

func adminOnlySourceRoutes() []adminRouteCase {
	return []adminRouteCase{
		{"create", http.MethodPost, "/v1/sources/create", `{"name":"Example","url":"https://example.com","url_rss":"https://example.com/rss"}`},
		{"update", http.MethodPut, "/v1/sources/01900000-0000-7000-8000-000000000010", `{"name":"Example","url":"https://example.com","url_rss":"https://example.com/rss"}`},
		{"delete", http.MethodDelete, "/v1/sources/01900000-0000-7000-8000-000000000010", ""},
		{"article-discovery", http.MethodGet, "/v1/sources/01900000-0000-7000-8000-000000000010/article-discovery", ""},
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
	// Generous timeout: when the guard lets a request through to the discovery handler, that handler
	// retries a failing feed with exponential backoff (~1.4s), past the default 1s.
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	return resp
}

func TestSourcesAdminRoutes_RejectRegularUser(t *testing.T) {
	requireNotProduction(t)

	for _, c := range adminOnlySourceRoutes() {
		t.Run(c.name, func(t *testing.T) {
			resp := doRequest(t, buildAppAsRegularUser(&mockSourceCtrl{}), c, authHeader(t))
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		})
	}
}

func TestSourcesAdminRoutes_RejectAnonymous(t *testing.T) {
	requireNotProduction(t)

	for _, c := range adminOnlySourceRoutes() {
		t.Run(c.name, func(t *testing.T) {
			resp := doRequest(t, defaultApp(), c, "")
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// TestSourcesAdminRoutes_AllowAdmin is the other half: the same requests must not be blocked when the
// database says the caller is an administrator. It asserts "not 401/403" rather than a specific
// success code because each route answers differently (201, 200, 204) and that is not what is under
// test here.
//
// The app is built with a mocked RSS client because one of these routes really does fetch a feed;
// with the default client that request would leave the machine and time out.
func TestSourcesAdminRoutes_AllowAdmin(t *testing.T) {
	requireNotProduction(t)

	for _, c := range adminOnlySourceRoutes() {
		t.Run(c.name, func(t *testing.T) {
			app := buildApp(&mockSourceCtrl{}, external.NewMockRSSClient(map[string]external.MockRSSResponse{}))
			resp := doRequest(t, app, c, authHeader(t))
			assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
			assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
		})
	}
}

// TestSourcesAdminRoutes_TokenClaimDoesNotGrantAccess is the test that pins the whole design: the
// caller presents a token whose `admin` claim is true, but the database row says otherwise. If
// authorization ever starts trusting the claim, this is what fails.
func TestSourcesAdminRoutes_TokenClaimDoesNotGrantAccess(t *testing.T) {
	requireNotProduction(t)

	resp := doRequest(t, buildAppAsRegularUser(&mockSourceCtrl{}), adminOnlySourceRoutes()[0], adminClaimHeader(t))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestSourcesAdminRoutes_DatabaseFailureIsNotADenial guards the inverse mistake: when the flag cannot
// be read the answer is 500, never 403. Telling an administrator they lost their access because the
// database blinked would send them chasing a permissions problem that does not exist.
func TestSourcesAdminRoutes_DatabaseFailureIsNotADenial(t *testing.T) {
	requireNotProduction(t)

	failing := &jwtmock.MockUserController{
		FindUserByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.User, error) {
			return db.User{}, assert.AnError
		},
	}

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	requireAdmin := middlewares.NewRequireAdminMiddleware(middlewares.NewAdminResolver(
		[]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, failing, fakeTxRunner,
	))
	app.Post("/v1/sources/create", adminChain(authMiddleware, requireAdmin, sourceendpoints.CreateSource(&mockSourceCtrl{}, fakeTxRunner))...)

	resp := doRequest(t, app, adminOnlySourceRoutes()[0], authHeader(t))
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
