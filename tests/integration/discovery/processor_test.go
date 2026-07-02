package discovery_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/discovery"
	"github.com/nathanap/news-feed-backend/services/judgment"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	_ "modernc.org/sqlite"
)

const sourceID = "01900000-0000-7000-8000-0000000000e1"

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func setup(t *testing.T, aiClient ai.Client) (controllers.TransactionRunner, db.Querier, *discovery.TreatmentProcessor) {
	t.Helper()
	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID: sourceID, Url: "https://src.com", UrlRss: "https://src.com/rss",
	})
	require.NoError(t, err)

	articleCtrl := controllers.NewArticleController()
	feedCtrl := controllers.NewFeedController()
	afCtrl := controllers.NewArticleFeedController()
	evaluator := judgment.NewEvaluator(aiClient, 70)
	processor := discovery.NewTreatmentProcessor(runTx, articleCtrl, feedCtrl, afCtrl, aiClient, aiClient, evaluator, false)
	return runTx, queries, processor
}

// seedUserWithFeed creates a user and one active feed with the given keywords, returning both ids.
func seedUserWithFeed(t *testing.T, queries db.Querier, keywords string) (userID, feedID string) {
	t.Helper()
	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID: user.ID, GoogleID: user.GoogleID, Email: user.Email, Name: user.Name, Picture: user.Picture,
	})
	require.NoError(t, err)

	feed := fixtures.NewTestFeed(user.ID)
	created, err := queries.CreateFeed(t.Context(), db.CreateFeedParams{
		ID: feed.ID, Name: feed.Name, Keywords: keywords, UserID: user.ID,
	})
	require.NoError(t, err)
	return user.ID, created.ID
}

func item(url string) discovery.DiscoveredArticle {
	return discovery.DiscoveredArticle{
		Title:       "Some News",
		Content:     "raw content",
		URLOriginal: url,
		SourceID:    sourceID,
	}
}

func TestIntegration_Processor_PersistsTreatedArticle(t *testing.T) {
	requireNotProduction(t)

	_, queries, processor := setup(t, &external.MockAIClient{})

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/a1")}))

	articles, err := queries.ListArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)
	assert.Equal(t, "treated: raw content", articles[0].Content) // from the mock's Treat
	assert.Equal(t, `["alpha","beta","gamma","delta","epsilon"]`, articles[0].Keywords)
	assert.Equal(t, "https://src.com/a1", articles[0].UrlOriginal)
}

func TestIntegration_Processor_SkipsExistingURL(t *testing.T) {
	requireNotProduction(t)

	_, queries, processor := setup(t, &external.MockAIClient{})

	// Pre-create the article, then process the same url → must not duplicate.
	_, err := queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID: "01900000-0000-7000-8000-0000000000e2", Title: "Existing", Content: "x",
		UrlOriginal: "https://src.com/dup", Keywords: `["a","b","c","d","e"]`, SourceID: sourceID,
	})
	require.NoError(t, err)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/dup")}))

	articles, err := queries.ListArticles(t.Context())
	require.NoError(t, err)
	assert.Len(t, articles, 1, "existing url_original must not be re-created")
}

func TestIntegration_Processor_JudgesIntoMatchingFeed(t *testing.T) {
	requireNotProduction(t)

	runTx, queries, processor := setup(t, &external.MockAIClient{})
	// Feed shares "alpha" with the mock's keywords (alpha..epsilon), so it survives layer 1; the
	// mock's Judge returns 90 (>= threshold 70), so it must be associated.
	userID, feedID := seedUserWithFeed(t, queries, `["alpha","rock","metal","music","concert"]`)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/judged")}))

	articles, err := queries.ListArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)

	var associations []db.ArticleFeed
	require.NoError(t, runTx(t.Context(), func(q db.Querier) error {
		var e error
		associations, e = q.FindArticleFeedsByArticleAndUser(t.Context(), db.FindArticleFeedsByArticleAndUserParams{
			UserID: userID, ArticleID: articles[0].ID,
		})
		return e
	}))
	require.Len(t, associations, 1, "matching feed above threshold must be associated")
	assert.Equal(t, feedID, associations[0].FeedID)
}

func TestIntegration_Processor_SkipsFeedBelowThreshold(t *testing.T) {
	requireNotProduction(t)

	// Judge returns 50, below the threshold of 70 → no association even though layer 1 matches.
	lowScore := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string, _ string) (int, error) {
			return 50, nil
		},
	}
	runTx, queries, processor := setup(t, lowScore)
	userID, _ := seedUserWithFeed(t, queries, `["alpha","rock","metal","music","concert"]`)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/lowscore")}))

	articles, err := queries.ListArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)

	var associations []db.ArticleFeed
	require.NoError(t, runTx(t.Context(), func(q db.Querier) error {
		var e error
		associations, e = q.FindArticleFeedsByArticleAndUser(t.Context(), db.FindArticleFeedsByArticleAndUserParams{
			UserID: userID, ArticleID: articles[0].ID,
		})
		return e
	}))
	assert.Empty(t, associations, "a feed scored below the threshold must not be associated")
}

func TestIntegration_Processor_AIFailureDoesNotPersist(t *testing.T) {
	requireNotProduction(t)

	failing := &external.MockAIClient{
		TreatFn: func(_ context.Context, _, _ string) (string, error) {
			return "", ai.ErrEmptyTreatment
		},
	}
	_, queries, processor := setup(t, failing)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/fail")}))

	articles, err := queries.ListArticles(t.Context())
	require.NoError(t, err)
	assert.Empty(t, articles, "a treatment failure must leave nothing persisted")
}
