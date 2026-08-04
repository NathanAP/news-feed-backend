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
	app.Get("/v1/articles/:id/translate", append(authMiddleware, articleendpoints.TranslateArticle(articleCtrl, controllers.NewArticleOutboundLinkController(), translator, runTx, ""))...)

	return app, queries
}

// seedUserWithNullTarget persists a user + refresh token and mints an access token whose
// language_to_translate preference is null, so the translate endpoint has no target to translate into.
func seedUserWithNullTarget(t *testing.T, queries db.Querier) string {
	t.Helper()
	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID:       user.ID,
		GoogleID: user.GoogleID,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	})
	require.NoError(t, err)

	rt := fixtures.NewTestRefreshToken(user.ID)
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID:        rt.ID,
		UserID:    rt.UserID,
		ExpiresAt: rt.ExpiresAt,
	})
	require.NoError(t, err)

	prefs := db.UserPreference{LanguageToTranslate: sql.NullString{Valid: false}, AiPersonality: "mixed"}
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID, prefs)
	require.NoError(t, err)
	return token
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
	token := seedUser(t, queries) // language_to_translate = pt (mock default)
	id := seedArticleWithLanguage(t, queries, "en")

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "pt", result["language"]) // target from the user's preference
	assert.Equal(t, "en", result["language_original"])
	assert.NotEmpty(t, result["title"])
	assert.NotEmpty(t, result["content"])
}

func TestIntegration_Translate_NullTarget(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupTranslateApp(t, &external.MockAIClient{})
	token := seedUserWithNullTarget(t, queries)
	id := seedArticleWithLanguage(t, queries, "en")

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	// No translation target configured in preferences → cannot translate.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Translate_SameLanguage(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupTranslateApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)
	id := seedArticleWithLanguage(t, queries, "pt") // matches the token's target (pt) → same language

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate", nil)
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

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id+"/translate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Translate_ArticleNotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupTranslateApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/non-existent/translate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestIntegration_Translate_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupTranslateApp(t, &external.MockAIClient{})

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/some-id/translate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
