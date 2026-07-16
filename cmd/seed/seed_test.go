package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

func strPtr(s string) *string { return &s }

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func testExamples() examples {
	return examples{
		User:            exampleUser{Email: "dev@test.local", Name: "Dev", Picture: ""},
		UserPreferences: exampleUserPreferences{LanguageToTranslate: strPtr("pt"), AIPersonality: "mixed"},
		Sources:         []exampleSource{{URL: "https://src.com", URLRss: "https://src.com/rss"}},
		Feeds:           []exampleFeed{{Name: "Feed A", Keywords: []string{"a", "b", "c", "d", "e"}}},
		Articles: []exampleArticle{
			{Title: "T1", Content: "C1", LanguageOriginal: "pt"},
			{Title: "T2", Content: "C2", LanguageOriginal: "en"},
		},
	}
}

func newTestSeedCtx(t *testing.T, ex examples) *seedCtx {
	t.Helper()
	database := testutils.SetupTestDB(t)
	runTx := controllers.NewTransactionRunner(database)
	refreshCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	userCtrl := controllers.NewUserController()
	prefCtrl := controllers.NewUserPreferencesController()
	authCtrl := controllers.NewAuthController(nil, userCtrl, refreshCtrl, prefCtrl, runTx, []byte("test-secret"), time.Hour)

	return &seedCtx{
		ctx:         context.Background(),
		runTx:       runTx,
		ex:          ex,
		userCtrl:    userCtrl,
		prefCtrl:    prefCtrl,
		refreshCtrl: refreshCtrl,
		sourceCtrl:  controllers.NewSourceController(),
		articleCtrl: controllers.NewArticleController(),
		feedCtrl:    controllers.NewFeedController(),
		afCtrl:      controllers.NewArticleFeedController(),
		authCtrl:    authCtrl,
	}
}

func TestRunDevUser_CreatesAndIsIdempotent(t *testing.T) {
	requireNotProduction(t)

	sc := newTestSeedCtx(t, testExamples())

	rep, err := runDevUser(sc)
	require.NoError(t, err)
	assert.Len(t, rep.created, 1)
	assert.Empty(t, rep.skipped)

	// The dev user exists with the example preferences.
	err = sc.runTx(sc.ctx, func(q db.Querier) error {
		user, err := sc.userCtrl.FindUserByGoogleID(sc.ctx, q, schemas.DevUserGoogleID)
		require.NoError(t, err)
		prefs, err := sc.prefCtrl.FindByUserID(sc.ctx, q, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "mixed", prefs.AiPersonality)
		return nil
	})
	require.NoError(t, err)

	// Second run skips.
	rep, err = runDevUser(sc)
	require.NoError(t, err)
	assert.Empty(t, rep.created)
	assert.Len(t, rep.skipped, 1)
}

func TestRunDevUser_InvalidPreferenceFails(t *testing.T) {
	requireNotProduction(t)

	ex := testExamples()
	ex.UserPreferences.AIPersonality = "friendly" // not a valid enum
	sc := newTestSeedCtx(t, ex)

	_, err := runDevUser(sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ai_personality")
}

func TestRunDevSources_Idempotent(t *testing.T) {
	requireNotProduction(t)

	sc := newTestSeedCtx(t, testExamples())

	rep, err := runDevSources(sc)
	require.NoError(t, err)
	assert.Len(t, rep.created, 1)

	rep, err = runDevSources(sc)
	require.NoError(t, err)
	assert.Empty(t, rep.created)
	assert.Len(t, rep.skipped, 1)
}

func TestRunDevArticles_RequiresSources(t *testing.T) {
	requireNotProduction(t)

	sc := newTestSeedCtx(t, testExamples())

	// No sources yet → clear error.
	_, err := runDevArticles(sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sources")

	// With sources, it creates one article per example (always, re-runnable).
	_, err = runDevSources(sc)
	require.NoError(t, err)

	rep, err := runDevArticles(sc)
	require.NoError(t, err)
	assert.Len(t, rep.created, 2)

	// Re-run creates again (fresh random urls, no collision).
	rep, err = runDevArticles(sc)
	require.NoError(t, err)
	assert.Len(t, rep.created, 2)

	var count int
	err = sc.runTx(sc.ctx, func(q db.Querier) error {
		articles, err := sc.articleCtrl.List(sc.ctx, q)
		count = len(articles)
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestRunDevFeeds_RequiresUserAndIsIdempotent(t *testing.T) {
	requireNotProduction(t)

	sc := newTestSeedCtx(t, testExamples())

	// No dev user → clear error.
	_, err := runDevFeeds(sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dev user")

	_, err = runDevUser(sc)
	require.NoError(t, err)

	rep, err := runDevFeeds(sc)
	require.NoError(t, err)
	assert.Len(t, rep.created, 1)

	rep, err = runDevFeeds(sc)
	require.NoError(t, err)
	assert.Len(t, rep.skipped, 1)
}

func TestRunDevArticleFeeds_LinksAndIsIdempotent(t *testing.T) {
	requireNotProduction(t)

	sc := newTestSeedCtx(t, testExamples())
	require.NoError(t, seedInOrder(sc))

	rep, err := runDevArticleFeeds(sc)
	require.NoError(t, err)
	assert.Len(t, rep.created, 2) // 2 articles × 1 feed

	// Re-run: everything already linked → all skipped.
	rep, err = runDevArticleFeeds(sc)
	require.NoError(t, err)
	assert.Empty(t, rep.created)
	assert.Len(t, rep.skipped, 2)
}

func TestRunDevLogin_MintsToken(t *testing.T) {
	requireNotProduction(t)

	sc := newTestSeedCtx(t, testExamples())
	_, err := runDevUser(sc)
	require.NoError(t, err)

	rep, err := runDevLogin(sc)
	require.NoError(t, err)
	assert.Len(t, rep.created, 1)
}

func TestRunDevLogin_RequiresUser(t *testing.T) {
	requireNotProduction(t)

	sc := newTestSeedCtx(t, testExamples())
	_, err := runDevLogin(sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dev user")
}

// seedInOrder runs the data seeds in dependency order (no login) for tests that need a populated DB.
func seedInOrder(sc *seedCtx) error {
	for _, run := range []func(*seedCtx) (*report, error){runDevUser, runDevSources, runDevFeeds, runDevArticles} {
		if _, err := run(sc); err != nil {
			return err
		}
	}
	return nil
}
