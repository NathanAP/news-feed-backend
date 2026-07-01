package articles_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/ai"
	articleendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/articles"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

func treatmentApp(aiClient ai.Client) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	// The mock implements both capabilities; wire it as treater and as every keyword mode.
	keyworders := map[string]ai.Keyworder{"local": aiClient, "groq": aiClient, "gemini": aiClient}
	app.Post("/v1/articles/treatment", append(authMiddleware, articleendpoints.TreatArticle(aiClient, keyworders, "local"))...)
	return app
}

func postTreatment(t *testing.T, app *fiber.App, body string, authed bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/v1/articles/treatment", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authed {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestTreatArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app := treatmentApp(&external.MockAIClient{})
	body := `{"article":{"title":"Some News","content":"raw body"}}`
	resp := postTreatment(t, app, body, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "treated: raw body", result["content"])
	assert.Len(t, result["keywords"].([]any), 5)
	assert.Equal(t, "local", result["keywords_mode"]) // default mode
	_, hasTreatMs := result["treatment_ms"]
	_, hasKwMs := result["keywords_ms"]
	assert.True(t, hasTreatMs && hasKwMs)
}

func TestTreatArticle_ModeOverride(t *testing.T) {
	requireNotProduction(t)

	app := treatmentApp(&external.MockAIClient{})
	body := `{"article":{"title":"Some News","content":"raw body"},"keywords_mode":"gemini"}`
	resp := postTreatment(t, app, body, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "gemini", result["keywords_mode"])
}

func TestTreatArticle_UnknownMode(t *testing.T) {
	requireNotProduction(t)

	app := treatmentApp(&external.MockAIClient{})
	body := `{"article":{"title":"Some News","content":"raw body"},"keywords_mode":"bogus"}`
	resp := postTreatment(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTreatArticle_MissingTitle(t *testing.T) {
	requireNotProduction(t)

	app := treatmentApp(&external.MockAIClient{})
	resp := postTreatment(t, app, `{"article":{"content":"raw body"}}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTreatArticle_MissingContent(t *testing.T) {
	requireNotProduction(t)

	app := treatmentApp(&external.MockAIClient{})
	resp := postTreatment(t, app, `{"article":{"title":"Some News"}}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTreatArticle_InvalidBody(t *testing.T) {
	requireNotProduction(t)

	app := treatmentApp(&external.MockAIClient{})
	resp := postTreatment(t, app, "not json", true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTreatArticle_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := treatmentApp(&external.MockAIClient{})
	resp := postTreatment(t, app, `{"article":{"title":"x","content":"y"}}`, false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTreatArticle_AIFailure(t *testing.T) {
	requireNotProduction(t)

	aiClient := &external.MockAIClient{
		TreatFn: func(_ context.Context, _, _ string) (string, error) {
			return "", ai.ErrEmptyTreatment
		},
	}
	app := treatmentApp(aiClient)
	resp := postTreatment(t, app, `{"article":{"title":"x","content":"y"}}`, true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
