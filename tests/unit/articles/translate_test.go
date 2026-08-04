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
	app.Get("/v1/articles/:id/translate", append(authMiddleware, articleendpoints.TranslateArticle(ctrl, &mockOutboundCtrl{}, translator, fakeTxRunner, ""))...)
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

// tokenWithLanguage builds an access token whose language_to_translate preference is the given value.
// Since 0.33.1 the endpoint reads the translation target from this preference (not from the URL).
func tokenWithLanguage(t *testing.T, language string) string {
	t.Helper()
	user := fixtures.NewTestUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	prefs := db.UserPreference{
		LanguageToTranslate: sql.NullString{String: language, Valid: true}, AiPersonality: "mixed",
	}
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID, prefs)
	require.NoError(t, err)
	return "Bearer " + token
}

// tokenWithNullLanguage builds an access token whose language_to_translate preference is null. The
// user then has no translation target configured, so the endpoint cannot translate (400).
func tokenWithNullLanguage(t *testing.T) string {
	t.Helper()
	user := fixtures.NewTestUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	prefs := db.UserPreference{
		LanguageToTranslate: sql.NullString{Valid: false}, AiPersonality: "mixed",
	}
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID, prefs)
	require.NoError(t, err)
	return "Bearer " + token
}

func getTranslate(t *testing.T, app *fiber.App, id, authHeaderValue string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate", nil)
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
	resp := getTranslate(t, app, "some-id", tokenWithLanguage(t, "es"))
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "es", result["language"])
	assert.Equal(t, "pt", result["language_original"])
	assert.Contains(t, result["title"], "translated:")
	assert.Contains(t, result["content"], "translated:")
}

// TestTranslateArticle_NullTarget proves the 0.33.1 model: the target comes from the user's
// language_to_translate preference, so a null preference leaves nothing to translate into (400).
func TestTranslateArticle_NullTarget(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", tokenWithNullLanguage(t))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTranslateArticle_UnsupportedLanguage(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", tokenWithLanguage(t, "xx"))
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
	resp := getTranslate(t, app, "missing", tokenWithLanguage(t, "es"))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTranslateArticle_UnknownOriginalLanguage(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage(""), &external.MockAIClient{}) // null language_original
	resp := getTranslate(t, app, "some-id", tokenWithLanguage(t, "es"))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTranslateArticle_SameLanguage(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", tokenWithLanguage(t, "pt")) // target == original
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
	resp := getTranslate(t, app, "some-id", tokenWithLanguage(t, "es"))
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestTranslateArticle_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := translateApp(articleCtrlWithLanguage("pt"), &external.MockAIClient{})
	resp := getTranslate(t, app, "some-id", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
