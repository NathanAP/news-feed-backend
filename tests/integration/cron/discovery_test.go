package cron_test

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	cronsvc "github.com/nathanap/news-feed-backend/services/cron"
	"github.com/nathanap/news-feed-backend/services/discovery"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	_ "modernc.org/sqlite"
)

const feedURL = "https://cron-feed.com/rss.xml"

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

// captureProcessor records what the runner hands it, so tests can assert what was discovered.
type captureProcessor struct {
	calls int
	got   []discovery.DiscoveredArticle
}

func (p *captureProcessor) Process(_ context.Context, articles []discovery.DiscoveredArticle) error {
	p.calls++
	p.got = append(p.got, articles...)
	return nil
}

func seedSource(t *testing.T, queries db.Querier, id, urlRss string) {
	t.Helper()
	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID:     id,
		Url:    "https://cron-feed.com",
		UrlRss: urlRss,
	})
	require.NoError(t, err)
}

func TestIntegration_DiscoveryRunner_DiscoversAndAdvancesWatermark(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	seedSource(t, queries, "01900000-0000-7000-8000-0000000000c1", feedURL)

	// Seed an old watermark so we can assert the run advances it at the end.
	_, err := queries.UpdateSystemLastArticleDiscovery(t.Context(), sql.NullTime{
		Time:  time.Date(2025, 1, 8, 0, 0, 0, 0, time.UTC),
		Valid: true,
	})
	require.NoError(t, err)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		feedURL: {StatusCode: http.StatusOK, Body: external.SampleRSSFeedDated},
	})
	proc := &captureProcessor{}
	runner := cronsvc.NewDiscoveryRunner(
		runTx,
		controllers.NewSourceController(),
		controllers.NewSystemController(),
		proc,
		mockClient,
		false,
	)

	require.NoError(t, runner.Run(t.Context()))

	// The CRON no longer filters by date — all feed items are handed to the processor, which is
	// where url_original dedup happens. SampleRSSFeedDated has 3 items.
	require.Equal(t, 1, proc.calls)
	assert.Len(t, proc.got, 3)

	// Watermark advanced past the old value.
	system, err := queries.GetSystem(t.Context())
	require.NoError(t, err)
	require.True(t, system.LastArticleDiscoveryAt.Valid)
	assert.True(t, system.LastArticleDiscoveryAt.Time.After(time.Date(2025, 1, 8, 0, 0, 0, 0, time.UTC)))

	// 0.19 persists nothing.
	articles, err := queries.ListArticles(t.Context())
	require.NoError(t, err)
	assert.Empty(t, articles)
}

func TestIntegration_DiscoveryRunner_SkipsWhenAppOff(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	seedSource(t, queries, "01900000-0000-7000-8000-0000000000c2", feedURL)
	require.NoError(t, fixtures.SetAppStatus(t.Context(), queries, false))

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		feedURL: {StatusCode: http.StatusOK, Body: external.SampleRSSFeedDated},
	})
	proc := &captureProcessor{}
	runner := cronsvc.NewDiscoveryRunner(
		runTx,
		controllers.NewSourceController(),
		controllers.NewSystemController(),
		proc,
		mockClient,
		false,
	)

	require.NoError(t, runner.Run(t.Context()))

	// Maintenance: the run is skipped entirely.
	assert.Equal(t, 0, proc.calls)
	system, err := queries.GetSystem(t.Context())
	require.NoError(t, err)
	assert.False(t, system.LastArticleDiscoveryAt.Valid, "watermark must not advance while app is off")
}
