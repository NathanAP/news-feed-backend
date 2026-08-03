package cron

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	robfigcron "github.com/robfig/cron/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/discovery"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// DiscoveryRunner executes one discovery sweep over every active source. The scheduler invokes it
// on each tick, and the test endpoint mirrors the same logic scoped to a single source.
type DiscoveryRunner struct {
	runTx       controllers.TransactionRunner
	sourceCtrl  controllers.SourceControllerInterface
	systemCtrl  controllers.SystemControllerInterface
	processor   discovery.Processor
	httpClient  *http.Client
	concurrency int
	// maxArticles caps how many discovered items a single sweep hands to the processor (a blunt
	// throttle to keep AI usage bounded while testing against a rate-limited provider). -1 = no cap.
	maxArticles int
	verbose     bool
}

func NewDiscoveryRunner(
	runTx controllers.TransactionRunner,
	sourceCtrl controllers.SourceControllerInterface,
	systemCtrl controllers.SystemControllerInterface,
	processor discovery.Processor,
	httpClient *http.Client,
	concurrency int,
	maxArticles int,
	verbose bool,
) *DiscoveryRunner {
	return &DiscoveryRunner{
		runTx:       runTx,
		sourceCtrl:  sourceCtrl,
		systemCtrl:  systemCtrl,
		processor:   processor,
		httpClient:  httpClient,
		concurrency: concurrency,
		maxArticles: maxArticles,
		verbose:     verbose,
	}
}

// Run performs a full discovery sweep: it reads the active sources and the watermark, fetches each
// feed (isolating per-source failures), hands the results to the processor, and finally advances
// the watermark. Network calls happen outside any transaction. While the app is under maintenance
// (app_status off) the run is skipped. Nothing is persisted in 0.19 — the processor is a no-op and
// the verbose log is the visual proof that the CRON ran and what it brought.
func (r *DiscoveryRunner) Run(ctx context.Context) error {
	now := time.Now().UTC()

	var (
		active  bool
		sources []db.Source
	)
	err := r.runTx(ctx, func(q db.Querier) error {
		system, err := r.systemCtrl.Get(ctx, q)
		if err != nil {
			return err
		}
		active = system.AppStatus
		if !active {
			return nil
		}
		sources, err = r.sourceCtrl.ListAll(ctx, q)
		return err
	})
	if err != nil {
		r.log(fmt.Sprintf("@@@ DISCOVERY ABORTED: %v @@@", err), logger.ColorRed)
		return err
	}
	if !active {
		r.log("@@@ DISCOVERY SKIPPED - app_status is off (maintenance) @@@", logger.ColorYellow)
		return nil
	}

	r.log(fmt.Sprintf("@@@ DISCOVERY START - %d source(s) @@@", len(sources)), logger.ColorYellow)

	all := r.fetchAll(ctx, sources)

	// Optional throttle: cap the batch handed to the processor. A blunt lever to keep AI usage under
	// a provider's rate limit while testing; -1 means no cap (the production value).
	if r.maxArticles >= 0 && len(all) > r.maxArticles {
		r.log(fmt.Sprintf("@@@ DISCOVERY CAP - processing %d of %d discovered item(s) (DISCOVERY_MAX_ARTICLES) @@@", r.maxArticles, len(all)), logger.ColorYellow)
		all = all[:r.maxArticles]
	}

	if err := r.processor.Process(ctx, all); err != nil {
		r.log(fmt.Sprintf("@@@ DISCOVERY PROCESSOR FAILED: %v @@@", err), logger.ColorRed)
		return err
	}

	if err := r.runTx(ctx, func(q db.Querier) error {
		_, err := r.systemCtrl.UpdateLastArticleDiscovery(ctx, q, now)
		return err
	}); err != nil {
		r.log(fmt.Sprintf("@@@ DISCOVERY WATERMARK UPDATE FAILED: %v @@@", err), logger.ColorRed)
		return err
	}

	r.log(fmt.Sprintf("@@@ DISCOVERY END - %d feed item(s) processed, watermark=%s @@@", len(all), now.Format(time.RFC3339)), logger.ColorGreen)
	return nil
}

// fetchAll pulls the RSS items from every source, isolating per-source failures (one bad feed must
// not abort the run). Sources are fetched by a bounded pool of at most `concurrency` workers; since
// each fetch is an independent network round-trip, this collapses the sweep from the sum of the
// feed latencies to their max (times ceil(n/concurrency)). concurrency <= 1 keeps it sequential and
// order-preserving (the default in dev/tests). No date lower bound is applied: deduplication by
// url_original (in the processor) is the reliable "is it new?" mechanism.
func (r *DiscoveryRunner) fetchAll(ctx context.Context, sources []db.Source) []discovery.DiscoveredArticle {
	workers := r.concurrency
	if workers < 1 {
		workers = 1
	}

	if workers == 1 || len(sources) <= 1 {
		all := make([]discovery.DiscoveredArticle, 0)
		for _, source := range sources {
			if items, ok := r.fetchOne(ctx, source); ok {
				all = append(all, items...)
			}
		}
		return all
	}

	// Bounded pool: the append is guarded by a mutex because workers merge their results
	// concurrently. Ordering of `all` is not significant — the processor treats articles as an
	// unordered set.
	var (
		all []discovery.DiscoveredArticle
		mu  sync.Mutex
		wg  sync.WaitGroup
	)
	sem := make(chan struct{}, workers)
	for _, source := range sources {
		select {
		case <-ctx.Done():
			wg.Wait()
			return all
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(source db.Source) {
			defer wg.Done()
			defer func() { <-sem }()
			if items, ok := r.fetchOne(ctx, source); ok {
				mu.Lock()
				all = append(all, items...)
				mu.Unlock()
			}
		}(source)
	}
	wg.Wait()
	return all
}

// fetchOne fetches a single source's feed, logging and reporting failure (ok=false) so the caller
// can drop it without aborting the sweep.
func (r *DiscoveryRunner) fetchOne(ctx context.Context, source db.Source) ([]discovery.DiscoveredArticle, bool) {
	items, err := discovery.DiscoverFromSource(ctx, r.httpClient, source, time.Time{})
	if err != nil {
		r.log(fmt.Sprintf("  source %s (%s) FAILED: %v", source.ID, source.UrlRss, err), logger.ColorRed)
		return nil, false
	}
	r.log(fmt.Sprintf("  source %s (%s): %d feed item(s)", source.ID, source.UrlRss, len(items)), logger.ColorCyan)
	return items, true
}

func (r *DiscoveryRunner) log(message string, color logger.Color) {
	if r.verbose {
		logger.Print(message, color)
	}
}

// Scheduler wraps the cron engine that ticks the discovery run on RSS_FEED_CRON_SCHEDULE.
type Scheduler struct {
	cron *robfigcron.Cron
}

// cronLogger routes the job-wrapper events below into the app's logger. It only ever fires on the two
// rare-but-important events those wrappers emit — a skipped tick (Info) and a recovered panic (Error) —
// so it adds no noise to a normal run. The engine's own logger is left at its default.
type cronLogger struct{}

func (cronLogger) Info(msg string, _ ...any) {
	logger.Log("@@@ CRON - "+msg+" @@@", logger.ColorYellow)
}

func (cronLogger) Error(err error, msg string, _ ...any) {
	logger.Log(fmt.Sprintf("@@@ CRON PANIC RECOVERED - %s: %v @@@", msg, err), logger.ColorRed)
}

// NewScheduler builds a scheduler that runs the discovery sweep on the given schedule (e.g.
// "@every 15m"). It does not start ticking until Start is called.
//
// Each tick is wrapped by two guards:
//   - SkipIfStillRunning: if a sweep is still running when the next tick fires, that tick is skipped
//     instead of starting a second sweep concurrently. Overlapping sweeps would NOT corrupt data (the
//     partial unique index on articles.url_original collapses a duplicate to a single row, and the
//     loser is dropped before judgement), but both would spend AI naming keywords for the same
//     articles — so this is a cost guard, not a correctness one.
//   - Recover: a panic inside a sweep is logged and swallowed instead of crashing the process. The
//     CRON runs in-process, so without this one bad sweep would take the whole API down, which the
//     project forbids ("exceções não devem derrubar a aplicação"). Fiber's recover covers HTTP
//     handlers only, never this goroutine.
func NewScheduler(schedule string, runner *DiscoveryRunner) (*Scheduler, error) {
	l := cronLogger{}
	c := robfigcron.New(robfigcron.WithChain(
		robfigcron.Recover(l),
		robfigcron.SkipIfStillRunning(l),
	))
	if _, err := c.AddFunc(schedule, func() {
		_ = runner.Run(context.Background())
	}); err != nil {
		return nil, fmt.Errorf("invalid cron schedule %q: %w", schedule, err)
	}
	return &Scheduler{cron: c}, nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}
