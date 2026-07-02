package articles_test

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	articleendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/articles"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

func translateApp(ctrl controllers.ArticleControllerInterface, translator ai.Translator) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	app.Get("/v1/articles/:id/translate/:language", append(authMiddleware, articleendpoints.TranslateArticle(ctrl, translator, fakeTxRunner))...)
	return app
}

// articleCtrlWithLanguage returns a mock whose FindByID yields an article with the given original
// language (empty = null).
func articleCtrlWithLanguage(languageOriginal string) *mockArticleCtrl {
	return &mockArticleCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Article, error) {
			a := fixtures.NewTestArticle()
			if languageOriginal != "" {
				a.LanguageOriginal = sql.NullString{String: languageOriginal, Valid: true}
			}
			return a, nil
		},
	}
}

// tokenWithTranslateDisabled builds an access token whose translate_content preference is false.
func tokenWithTranslateDisabled(t *testing.T) string {
	t.Helper()
	user := fixtures.NewTestUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	prefs := db.UserPreference{
		Theme: "dark", Language: "pt", TranslateContent: 0, AiPersonality: "mixed",
	}
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID, prefs)
	require.NoError(t, err)
	return "Bearer " + token
}

func getTranslate(t *testing.T, app *fiber.App, id, language, authHeaderValue string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate/"+language, nil)
	require.NoError(t, err)
	if authHeaderValue != "" {
		req.Header.Set("Authorization", authHeaderValue)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestTranslateArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", "es", authHeader(t))
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "es", result["language"])
	assert.Equal(t, "pt", result["language_original"])
	assert.Contains(t, result["title"], "translated:")
	assert.Contains(t, result["content"], "translated:")
}

func TestTranslateArticle_GateDisabled(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", "es", tokenWithTranslateDisabled(t))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestTranslateArticle_UnsupportedLanguage(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", "xx", authHeader(t))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTranslateArticle_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Article, error) {
			return db.Article{}, controllers.ErrArticleNotFound
		},
	}
	app := translateApp(ctrl, &external.MockAIClient{})
	resp := getTranslate(t, app, "missing", "es", authHeader(t))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTranslateArticle_UnknownOriginalLanguage(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage(""), &external.MockAIClient{}) // null language_original
	resp := getTranslate(t, app, "some-id", "es", authHeader(t))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTranslateArticle_SameLanguage(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", "pt", authHeader(t)) // target == original
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTranslateArticle_AIFailure(t *testing.T) {
	requireNotProduction(t)

	failing := &external.MockAIClient{
		TranslateFn: func(_ context.Context, _, _, _, _ string) (ai.Translation, error) {
			return ai.Translation{}, ai.ErrInvalidTranslation
		},
	}
	app := translateApp(articleCtrlWithLanguage("pt"), failing)
	resp := getTranslate(t, app, "some-id", "es", authHeader(t))
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestTranslateArticle_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", "es", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
