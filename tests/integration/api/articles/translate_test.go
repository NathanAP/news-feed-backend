package articles_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

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
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

func setupTranslateApp(t *testing.T, translator ai.Translator) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	articleCtrl := controllers.NewArticleController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/v1/articles/:id/translate/:language", append(authMiddleware, articleendpoints.TranslateArticle(articleCtrl, translator, runTx))...)

	return app, queries
}

// seedArticleWithLanguage inserts an active article with the given original language (empty = null),
// returning its id. It needs an active source, so one is seeded first.
func seedArticleWithLanguage(t *testing.T, queries db.Querier, languageOriginal string) string {
	t.Helper()
	sourceID := seedSource(t, queries)
	article := fixtures.NewTestArticle()
	lang := sql.NullString{}
	if languageOriginal != "" {
		lang = sql.NullString{String: languageOriginal, Valid: true}
	}
	created, err := queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID:               article.ID,
		Title:            article.Title,
		Content:          article.Content,
		UrlOriginal:      article.UrlOriginal,
		Keywords:         article.Keywords,
		SourceID:         sourceID,
		LanguageOriginal: lang,
	})
	require.NoError(t, err)
	return created.ID
}

func TestIntegration_Translate_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupTranslateApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)
	id := seedArticleWithLanguage(t, queries, "pt")

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate/es", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "es", result["language"])
	assert.Equal(t, "pt", result["language_original"])
	assert.NotEmpty(t, result["title"])
	assert.NotEmpty(t, result["content"])
}

func TestIntegration_Translate_SameLanguage(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupTranslateApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)
	id := seedArticleWithLanguage(t, queries, "pt")

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate/pt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Translate_UnknownOriginalLanguage(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupTranslateApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)
	id := seedArticleWithLanguage(t, queries, "") // null language_original

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate/es", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Translate_ArticleNotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupTranslateApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/non-existent/translate/es", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestIntegration_Translate_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupTranslateApp(t, &external.MockAIClient{})

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/some-id/translate/es", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
