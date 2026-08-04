package discovery_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/discovery"
	"github.com/nathanap/news-feed-backend/services/judgement"
	"github.com/nathanap/news-feed-backend/services/outboundlinks"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	servicemocks "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

const sourceID = "01900000-0000-7000-8000-0000000000e1"

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

// setup builds a processor with concurrency 1: the existing assertions rely on deterministic,
// sequential processing.
func setup(t *testing.T, aiClient ai.Client) (controllers.TransactionRunner, db.Querier, *discovery.TreatmentProcessor) {
	t.Helper()
	return setupWithConcurrency(t, aiClient, 1)
}

func setupWithConcurrency(t *testing.T, aiClient ai.Client, concurrency int) (controllers.TransactionRunner, db.Querier, *discovery.TreatmentProcessor) {
	t.Helper()
	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID: sourceID, Name: "Test Source", Url: "https://src.com", UrlRss: "https://src.com/rss",
	})
	require.NoError(t, err)

	articleCtrl := controllers.NewArticleController()
	outboundCtrl := controllers.NewArticleOutboundLinkController()
	feedCtrl := controllers.NewFeedController()
	afCtrl := controllers.NewArticleFeedController()
	evaluator := judgement.NewEvaluator(aiClient, 70, 0.30, 2)
	detector := &servicemocks.MockLanguageDetector{}
	// clientURL empty: url treatment is skipped for these tests (they assert on other behavior).
	processor := discovery.NewTreatmentProcessor(runTx, articleCtrl, outboundCtrl, feedCtrl, afCtrl, detector, aiClient, evaluator, "", false, concurrency, -1, false)
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
		ID: feed.ID, Name: feed.Name, Keywords: json.RawMessage(keywords), UserID: user.ID,
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

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)
	assert.Equal(t, "raw content", articles[0].Content) // sanitized raw body (no HTML to strip → unchanged)
	// JSONEq, not Equal: keywords are JSONB, and Postgres stores the parsed value rather than the
	// text it was given — it re-renders on read (notably with a space after each comma), so the exact
	// bytes are not ours to assert on. What matters is that the same JSON array came back.
	assert.JSONEq(t, `["alpha","beta","gamma","delta","epsilon"]`, string(articles[0].Keywords))
	assert.Equal(t, "https://src.com/a1", articles[0].UrlOriginal)
	require.True(t, articles[0].LanguageOriginal.Valid, "detected language must be persisted")
	assert.Equal(t, "pt", articles[0].LanguageOriginal.String) // from the mock detector
}

func TestIntegration_Processor_NullLanguageOnDetectionFailure(t *testing.T) {
	requireNotProduction(t)

	// Detector reports failure → language_original must be stored as null, without breaking persistence.
	failingDetector := &servicemocks.MockLanguageDetector{
		DetectFn: func(_, _ string) (string, bool) { return "", false },
	}
	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)
	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{ID: sourceID, Name: "Test Source", Url: "https://src.com", UrlRss: "https://src.com/rss"})
	require.NoError(t, err)

	aiClient := &external.MockAIClient{}
	evaluator := judgement.NewEvaluator(aiClient, 70, 0.30, 2)
	processor := discovery.NewTreatmentProcessor(
		runTx, controllers.NewArticleController(), controllers.NewArticleOutboundLinkController(), controllers.NewFeedController(),
		controllers.NewArticleFeedController(), failingDetector, aiClient, evaluator, "", false, 1, -1, false,
	)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/nolang")}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)
	assert.False(t, articles[0].LanguageOriginal.Valid, "a failed detection must persist a null language_original")
}

func TestIntegration_Processor_RewritesInternalLinks(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)
	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{ID: sourceID, Name: "Test Source", Url: "https://src.com", UrlRss: "https://src.com/rss"})
	require.NoError(t, err)

	// An article we already have; the newly discovered content links to its url_original.
	const existingID = "01900000-0000-7000-8000-0000000000f1"
	_, err = queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID: existingID, Title: "Existing", Content: "x",
		UrlOriginal: "https://src.com/existing", Keywords: json.RawMessage(`["a","b","c","d","e"]`), SourceID: sourceID,
	})
	require.NoError(t, err)

	aiClient := &external.MockAIClient{}
	evaluator := judgement.NewEvaluator(aiClient, 70, 0.30, 2)
	processor := discovery.NewTreatmentProcessor(
		runTx, controllers.NewArticleController(), controllers.NewArticleOutboundLinkController(), controllers.NewFeedController(),
		controllers.NewArticleFeedController(), &servicemocks.MockLanguageDetector{}, aiClient, evaluator,
		"https://client.app", false, 1, -1, false,
	)

	newItem := discovery.DiscoveredArticle{
		Title:       "Linking News",
		Content:     `<p>see <a href="https://src.com/existing">ours</a> and <a href="https://ext.com/z">external</a></p>`,
		URLOriginal: "https://src.com/linking",
		SourceID:    sourceID,
	}
	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{newItem}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	var linkingID, content string
	for _, a := range articles {
		if a.UrlOriginal == "https://src.com/linking" {
			linkingID, content = a.ID, a.Content
		}
	}
	require.NotEmpty(t, content, "the linking article must have been persisted")

	// 0.45: the stored body no longer carries URLs — every anchor holds an outbound-link id.
	assert.NotContains(t, content, `href="https://`, "stored body must not carry raw URLs (only outbound ids)")

	// The outbound rows hold the real targets: the internal link as the {CLIENT_URL} token, the
	// external one literally.
	links, err := queries.ListArticleOutboundLinksByArticleIDs(t.Context(), []string{linkingID})
	require.NoError(t, err)
	hrefs := make([]string, len(links))
	for i, l := range links {
		hrefs[i] = l.Href
	}
	assert.ElementsMatch(t, []string{"{CLIENT_URL}/articles/" + existingID, "https://ext.com/z"}, hrefs,
		"internal link stored as token, external stored literally")

	// The read-time swap round-trips: ids back to real hrefs, {CLIENT_URL} expanded to the client URL.
	resolved := outboundlinks.Resolve(content, controllers.OutboundLinksByID(links), "https://client.app")
	assert.Contains(t, resolved, `href="https://client.app/articles/`+existingID+`"`, "internal link resolves to the client URL")
	assert.Contains(t, resolved, `href="https://ext.com/z"`, "external link resolves back unchanged")
}

// TestIntegration_Processor_RetroactivelyLinksLaterArticle proves the 0.45 "Alteração de URLs de
// outras notícias" step: an older article linking an external URL is retro-linked when a later article
// arrives as that URL. The older article's body is never touched — only its outbound row is retargeted.
func TestIntegration_Processor_RetroactivelyLinksLaterArticle(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)
	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{ID: sourceID, Name: "Test Source", Url: "https://src.com", UrlRss: "https://src.com/rss"})
	require.NoError(t, err)

	aiClient := &external.MockAIClient{}
	evaluator := judgement.NewEvaluator(aiClient, 70, 0.30, 2)
	processor := discovery.NewTreatmentProcessor(
		runTx, controllers.NewArticleController(), controllers.NewArticleOutboundLinkController(), controllers.NewFeedController(),
		controllers.NewArticleFeedController(), &servicemocks.MockLanguageDetector{}, aiClient, evaluator,
		"https://client.app", false, 1, -1, false,
	)

	// Article A links a URL that is not ours yet — stored as an external outbound href.
	articleA := discovery.DiscoveredArticle{
		Title:       "Early News",
		Content:     `<p><a href="https://src.com/future">a story to come</a></p>`,
		URLOriginal: "https://src.com/a",
		SourceID:    sourceID,
	}
	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{articleA}))

	// Article B arrives AS that URL. Its arrival must retarget A's row to B's internal link.
	articleB := discovery.DiscoveredArticle{
		Title:       "The Future Story",
		Content:     `<p>here it is</p>`,
		URLOriginal: "https://src.com/future",
		SourceID:    sourceID,
	}
	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{articleB}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	var aID, bID string
	for _, a := range articles {
		switch a.UrlOriginal {
		case "https://src.com/a":
			aID = a.ID
		case "https://src.com/future":
			bID = a.ID
		}
	}
	require.NotEmpty(t, aID)
	require.NotEmpty(t, bID)

	links, err := queries.ListArticleOutboundLinksByArticleIDs(t.Context(), []string{aID})
	require.NoError(t, err)
	require.Len(t, links, 1, "A has one outbound link")
	assert.Equal(t, "{CLIENT_URL}/articles/"+bID, links[0].Href,
		"A's external link was retargeted to B's internal (token) URL when B arrived")
}

func TestIntegration_Processor_SkipsExistingURL(t *testing.T) {
	requireNotProduction(t)

	_, queries, processor := setup(t, &external.MockAIClient{})

	// Pre-create the article, then process the same url → must not duplicate.
	_, err := queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID: "01900000-0000-7000-8000-0000000000e2", Title: "Existing", Content: "x",
		UrlOriginal: "https://src.com/dup", Keywords: json.RawMessage(`["a","b","c","d","e"]`), SourceID: sourceID,
	})
	require.NoError(t, err)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/dup")}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	assert.Len(t, articles, 1, "existing url_original must not be re-created")
}

// articleFeedAssociations returns the articles_feeds rows for a (user, article) pair.
func articleFeedAssociations(t *testing.T, runTx controllers.TransactionRunner, userID, articleID string) []db.ArticlesFeed {
	t.Helper()
	var associations []db.ArticlesFeed
	require.NoError(t, runTx(t.Context(), func(q db.Querier) error {
		var e error
		associations, e = q.FindArticleFeedsByArticleAndUser(t.Context(), db.FindArticleFeedsByArticleAndUserParams{
			UserID: userID, ArticleID: articleID,
		})
		return e
	}))
	return associations
}

// The mock keyworder emits alpha..epsilon (5 keywords), so feeds below are crafted to land in a
// specific triage band against those.

func TestIntegration_Processor_AutoAssociatesStrongOverlap(t *testing.T) {
	requireNotProduction(t)

	// Feed shares 3 of its 5 keywords (alpha,beta,gamma) → 60% >= 30% → auto-associated WITHOUT AI.
	// The Judge would error if called, proving no AI ran.
	noAI := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			return 0, assert.AnError
		},
	}
	runTx, queries, processor := setup(t, noAI)
	userID, feedID := seedUserWithFeed(t, queries, `["alpha","beta","gamma","music","concert"]`)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/auto")}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)

	associations := articleFeedAssociations(t, runTx, userID, articles[0].ID)
	require.Len(t, associations, 1, "a strong-overlap feed must be auto-associated without AI")
	assert.Equal(t, feedID, associations[0].FeedID)
}

func TestIntegration_Processor_JudgesBorderlineIntoFeed(t *testing.T) {
	requireNotProduction(t)

	runTx, queries, processor := setup(t, &external.MockAIClient{}) // default Judge score 90
	// 7-keyword feed sharing alpha,beta (2/7 = 0.28 < 30%, overlap 2 >= min 2) → borderline → AI judges.
	userID, feedID := seedUserWithFeed(t, queries, `["alpha","beta","k1","k2","k3","k4","k5"]`)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/judged")}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)

	associations := articleFeedAssociations(t, runTx, userID, articles[0].ID)
	require.Len(t, associations, 1, "a borderline feed judged above threshold must be associated")
	assert.Equal(t, feedID, associations[0].FeedID)
}

func TestIntegration_Processor_SkipsFeedBelowThreshold(t *testing.T) {
	requireNotProduction(t)

	// Judge returns 50, below the threshold of 70 → no association for the borderline feed.
	lowScore := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			return 50, nil
		},
	}
	runTx, queries, processor := setup(t, lowScore)
	userID, _ := seedUserWithFeed(t, queries, `["alpha","beta","k1","k2","k3","k4","k5"]`) // borderline

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/lowscore")}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)

	assert.Empty(t, articleFeedAssociations(t, runTx, userID, articles[0].ID), "a feed scored below the threshold must not be associated")
}

func TestIntegration_Processor_DiscardsWeakOverlap(t *testing.T) {
	requireNotProduction(t)

	// Feed shares only 1 keyword (alpha) → below min matches (2) → discarded WITHOUT AI, no association.
	noAI := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			return 0, assert.AnError
		},
	}
	runTx, queries, processor := setup(t, noAI)
	userID, _ := seedUserWithFeed(t, queries, `["alpha","rock","metal","music","concert"]`)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/weak")}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)

	assert.Empty(t, articleFeedAssociations(t, runTx, userID, articles[0].ID), "a 1-keyword overlap must be discarded without AI")
}

func TestIntegration_Processor_ConcurrentPersistsAllArticles(t *testing.T) {
	requireNotProduction(t)

	// A batch of distinct articles processed by a pool of 4 workers must all be persisted — the
	// worker pool must not drop or lose any article.
	_, queries, processor := setupWithConcurrency(t, &external.MockAIClient{}, 4)

	const n = 12
	batch := make([]discovery.DiscoveredArticle, 0, n)
	for i := 0; i < n; i++ {
		batch = append(batch, item(fmt.Sprintf("https://src.com/c%d", i)))
	}

	require.NoError(t, processor.Process(t.Context(), batch))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	assert.Len(t, articles, n, "every distinct article in the batch must be persisted under concurrency")
}

func TestIntegration_Processor_ConcurrentDedupsWithinBatch(t *testing.T) {
	requireNotProduction(t)

	// The same url_original appears several times in one batch. With concurrent workers the dedup
	// check may pass for more than one before any insert lands, so the url_original unique
	// constraint is the real guarantee — the batch must collapse to a single persisted row.
	_, queries, processor := setupWithConcurrency(t, &external.MockAIClient{}, 4)

	batch := []discovery.DiscoveredArticle{
		item("https://src.com/same"), item("https://src.com/same"),
		item("https://src.com/same"), item("https://src.com/same"),
	}

	require.NoError(t, processor.Process(t.Context(), batch))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	assert.Len(t, articles, 1, "duplicate url_original within a batch must collapse to one row")
}

func TestIntegration_Processor_AIFailureDoesNotPersist(t *testing.T) {
	requireNotProduction(t)

	// The body treatment no longer calls AI; the remaining AI step is keyword naming. A keyword
	// failure must abort that article, leaving nothing persisted (it is rediscovered next run).
	failing := &external.MockAIClient{
		KeywordsFn: func(_ context.Context, _, _ string) ([]string, error) {
			return nil, ai.ErrInvalidKeywords
		},
	}
	_, queries, processor := setup(t, failing)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/fail")}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	assert.Empty(t, articles, "a keyword failure must leave nothing persisted")
}

// TestIntegration_Processor_SkipsInactiveUserFeed exercises the 0.43 filter through the WHOLE pipeline
// (mocked AI, real DB), not just the query: two users own an identical auto-associate feed, one active
// and one backdated past the window. Running discovery over one article must associate it to the active
// user's feed and skip the inactive one's — proving the processor threads inactiveDays into judgement.
func TestIntegration_Processor_SkipsInactiveUserFeed(t *testing.T) {
	requireNotProduction(t)

	// Auto-associate band (3/5 overlap with the mock keyworder's alpha..epsilon) → no AI. The Judge
	// errors if reached, so any association here is pre-AI and purely the keyword+activity filter.
	noAI := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			return 0, assert.AnError
		},
	}

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)
	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID: sourceID, Name: "Test Source", Url: "https://src.com", UrlRss: "https://src.com/rss",
	})
	require.NoError(t, err)

	// inactiveDays = 15: the pipeline must drop feeds of users not seen within 15 days.
	processor := discovery.NewTreatmentProcessor(
		runTx, controllers.NewArticleController(), controllers.NewArticleOutboundLinkController(), controllers.NewFeedController(),
		controllers.NewArticleFeedController(), &servicemocks.MockLanguageDetector{}, noAI,
		judgement.NewEvaluator(noAI, 70, 0.30, 2), "", false, 1, 15, false,
	)

	const keywords = `["alpha","beta","gamma","music","concert"]`
	activeUser, activeFeed := seedDiscoveryUserWithFeed(t, queries, "a1", keywords)
	inactiveUser, _ := seedDiscoveryUserWithFeed(t, queries, "b2", keywords)
	// Backdate the second user well past the window.
	_, err = database.ExecContext(context.Background(),
		"UPDATE users SET last_active_at = $1 WHERE id = $2",
		time.Now().UTC().AddDate(0, 0, -40), inactiveUser)
	require.NoError(t, err)

	require.NoError(t, processor.Process(t.Context(), []discovery.DiscoveredArticle{item("https://src.com/inactivefilter")}))

	articles, err := queries.ListAllArticles(t.Context())
	require.NoError(t, err)
	require.Len(t, articles, 1)

	active := articleFeedAssociations(t, runTx, activeUser, articles[0].ID)
	require.Len(t, active, 1, "active user's feed must be associated")
	assert.Equal(t, activeFeed, active[0].FeedID)

	inactive := articleFeedAssociations(t, runTx, inactiveUser, articles[0].ID)
	assert.Empty(t, inactive, "inactive user's feed must be skipped by the pipeline")
}

// seedDiscoveryUserWithFeed is seedUserWithFeed with a 2-hex suffix, so two distinct users (and feeds)
// can coexist in one test without colliding on the fixed fixture ids.
func seedDiscoveryUserWithFeed(t *testing.T, queries db.Querier, suffix, keywords string) (userID, feedID string) {
	t.Helper()
	user := fixtures.NewTestUser()
	user.ID = "01900000-0000-7000-8000-0000000000" + suffix
	user.GoogleID = "google-disc-" + suffix
	user.Email = suffix + "-disc@example.com"
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID: user.ID, GoogleID: user.GoogleID, Email: user.Email, Name: user.Name, Picture: user.Picture,
	})
	require.NoError(t, err)

	feed := fixtures.NewTestFeed(user.ID)
	feed.ID = "019000fd-0000-7000-8000-0000000000" + suffix
	created, err := queries.CreateFeed(t.Context(), db.CreateFeedParams{
		ID: feed.ID, Name: feed.Name, Keywords: json.RawMessage(keywords), UserID: user.ID,
	})
	require.NoError(t, err)
	return user.ID, created.ID
}
