package discovery

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/embedtreatment"
	"github.com/nathanap/news-feed-backend/services/judgement"
	"github.com/nathanap/news-feed-backend/services/langdetect"
	"github.com/nathanap/news-feed-backend/services/outboundlinks"
	"github.com/nathanap/news-feed-backend/services/sanitize"
	"github.com/nathanap/news-feed-backend/services/urltreatment"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// TreatmentProcessor is the real discovery sink: for each discovered article it deduplicates by
// url_original, detects the original language, rewrites in-content links that point to articles we
// already have (url treatment), sanitizes the raw RSS body to the safe-HTML whitelist (bluemonday,
// deterministic — the AI never touches the body), names keywords over the sanitized content,
// persists the result (rewriting in-body anchors to article_outbound_links ids and retro-linking older
// articles, see persist), and finally judges the article against the user feeds — writing an
// articles_feeds association for every feed that clears the threshold. AI calls happen outside any
// transaction. A failure on one article (AI error, persistence error) is logged and skipped — since
// nothing is persisted for it, the article is simply rediscovered and retried on the next run.
// Judgement is best-effort and non-retroactive: once the article is persisted, a judgement failure
// is logged and skipped, and the article is not re-judged on later runs (dedup skips it).
//
// Articles within a batch are processed by a bounded worker pool (concurrency): the per-article
// pipeline is dominated by network latency (2-3 AI round-trips), so running several articles at once
// is the lever that turns discovery throughput. concurrency <= 1 keeps the run strictly sequential
// (the default, used in dev and tests for determinism); staging/production raise it via
// DISCOVERY_CONCURRENCY, bounded by the AI provider's rate limit. The in-process pool is the
// single-node case of the future distributed queue + workers model (see ROADMAP), so the per-article
// pipeline stays behind processOne and does not know who dispatches it.
type TreatmentProcessor struct {
	runTx        controllers.TransactionRunner
	articleCtrl  controllers.ArticleControllerInterface
	outboundCtrl controllers.ArticleOutboundLinkControllerInterface
	feedCtrl     controllers.FeedControllerInterface
	afCtrl       controllers.ArticleFeedControllerInterface
	detector     langdetect.Detector
	keyworder    ai.Keyworder
	evaluator    *judgement.Evaluator
	// clientURL is the base URL of the web client (e.g. https://app.example.com); internal article
	// links become clientURL + "/articles/" + id. Empty disables url treatment (links are only sanitized).
	clientURL   string
	urlVerbose  bool
	concurrency int
	// inactiveDays is passed to judgement layer 1 so feeds of users not seen within that many days are
	// skipped; -1 disables the filter (every user counts as active).
	inactiveDays int
	verbose      bool
}

func NewTreatmentProcessor(
	runTx controllers.TransactionRunner,
	articleCtrl controllers.ArticleControllerInterface,
	outboundCtrl controllers.ArticleOutboundLinkControllerInterface,
	feedCtrl controllers.FeedControllerInterface,
	afCtrl controllers.ArticleFeedControllerInterface,
	detector langdetect.Detector,
	keyworder ai.Keyworder,
	evaluator *judgement.Evaluator,
	clientURL string,
	urlVerbose bool,
	concurrency int,
	inactiveDays int,
	verbose bool,
) *TreatmentProcessor {
	return &TreatmentProcessor{
		runTx:        runTx,
		articleCtrl:  articleCtrl,
		outboundCtrl: outboundCtrl,
		feedCtrl:     feedCtrl,
		afCtrl:       afCtrl,
		detector:     detector,
		keyworder:    keyworder,
		evaluator:    evaluator,
		clientURL:    clientURL,
		urlVerbose:   urlVerbose,
		concurrency:  concurrency,
		inactiveDays: inactiveDays,
		verbose:      verbose,
	}
}

var _ Processor = (*TreatmentProcessor)(nil)

// Process runs the per-article pipeline over the batch. Each article is independent (dedup by
// url_original, then treat/keyword/detect/persist/judge), so they are dispatched to a bounded pool
// of at most `concurrency` workers. A per-article failure is logged and dropped inside processOne;
// nothing here aborts the batch, so Process always returns nil — the run is best-effort and dropped
// articles are simply rediscovered next time.
func (p *TreatmentProcessor) Process(ctx context.Context, articles []DiscoveredArticle) error {
	workers := p.concurrency
	if workers < 1 {
		workers = 1
	}

	// Sequential fast path: keeps the default (dev/tests) free of any goroutine scheduling and its
	// interleaved logs, and preserves the original processing order.
	if workers == 1 || len(articles) <= 1 {
		for _, article := range articles {
			p.processOne(ctx, article)
		}
		return nil
	}

	// Bounded worker pool: the semaphore caps in-flight articles at `workers`; each acquires a slot
	// before starting and releases it when done. ctx cancellation (shutdown) stops dispatching new
	// work and we wait for the in-flight ones to unwind.
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for _, article := range articles {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(article DiscoveredArticle) {
			defer wg.Done()
			defer func() { <-sem }()
			p.processOne(ctx, article)
		}(article)
	}
	wg.Wait()
	return nil
}

// processOne runs the full pipeline for a single discovered article. It is self-contained and
// safe to run concurrently with itself: the dedup check plus the url_original unique constraint
// make a duplicate within the same batch collapse to a single persisted row (the losing insert
// gets ErrArticleAlreadyExists and is dropped), and every write goes through its own short
// transaction. Any failure is logged and swallowed so one bad article never affects the others.
func (p *TreatmentProcessor) processOne(ctx context.Context, article DiscoveredArticle) {
	exists, err := p.alreadyExists(ctx, article.URLOriginal)
	if err != nil {
		p.log(fmt.Sprintf("    dedup check failed for %s: %v", article.URLOriginal, err), logger.ColorRed)
		return
	}
	if exists {
		p.log(fmt.Sprintf("    skip (already exists): %s", article.URLOriginal), logger.ColorBlue)
		return
	}

	// Detect the original language from the raw (unsanitized) text. This is a lingua-go call, not
	// AI: deterministic and offline. A failed detection is not fatal — the article is stored with
	// a null language_original (it just cannot be translated later).
	languageOriginal := p.detectLanguage(article.Title, article.Content)

	// URL treatment: rewrite in-content links that point to articles we already have so they open in
	// our client. Runs before sanitize (the rewritten links must survive the whitelist). Best-effort
	// and non-fatal — on error the original body flows on unchanged. Skipped entirely without a
	// configured client URL.
	body := article.Content
	if p.clientURL != "" {
		treated, terr := urltreatment.Treat(ctx, article.Content, p.resolveInternalURLs, p.urlVerbose)
		if terr != nil {
			p.log(fmt.Sprintf("    url treatment failed for %s: %v (using original content)", article.URLOriginal, terr), logger.ColorRed)
		}
		body = treated
	}

	// Embed treatment: convert script embeds (Instagram) to links and fix Twitch iframe `parent` for
	// our client, before sanitize. Deterministic (no AI, no DB) and best-effort — on error the body
	// flows on unchanged.
	if treated, eerr := embedtreatment.Treat(body, p.clientURL, p.urlVerbose); eerr != nil {
		p.log(fmt.Sprintf("    embed treatment failed for %s: %v (using original content)", article.URLOriginal, eerr), logger.ColorRed)
	} else {
		body = treated
	}

	// Clean the (url- and embed-treated) RSS body to the safe-HTML whitelist. Deterministic (no AI,
	// no error): the body is never rewritten by a model, only stripped of unsafe/unknown markup.
	content := sanitize.Sanitize(body)

	// Keywords run over the plain text (markup stripped): the model does not need the HTML and it
	// keeps the input tokens down. The stored article keeps the full sanitized HTML (content above).
	keywords, err := p.keyworder.Keywords(ctx, article.Title, sanitize.PlainText(content))
	if err != nil {
		p.log(fmt.Sprintf("    keywords failed for %s: %v", article.URLOriginal, err), logger.ColorRed)
		return
	}

	saved, err := p.persist(ctx, article, content, keywords, languageOriginal)
	if err != nil {
		if errors.Is(err, controllers.ErrArticleAlreadyExists) {
			return // created concurrently between the dedup check and the insert
		}
		p.log(fmt.Sprintf("    persist failed for %s: %v", article.URLOriginal, err), logger.ColorRed)
		return
	}

	p.log(fmt.Sprintf("    saved: %s (%s)", article.Title, article.URLOriginal), logger.ColorGreen)

	// Judgement is best-effort: the article is already persisted, so a failure here must not
	// abort the run — it is logged and the next article proceeds.
	p.judge(ctx, saved, keywords)
}

// judge runs the two judgement layers for a freshly persisted article and writes an articles_feeds
// association for every feed that clears the threshold. Layer 1 (candidate feeds) and the winner
// inserts run in their own transactions; the AI scoring (layer 2) runs in between, outside any
// transaction.
func (p *TreatmentProcessor) judge(ctx context.Context, article db.Article, keywords []string) {
	var candidates []controllers.FeedCandidate
	if err := p.runTx(ctx, func(q db.Querier) error {
		var e error
		candidates, e = p.feedCtrl.FindCandidatesByKeywords(ctx, q, keywords, p.inactiveDays)
		return e
	}); err != nil {
		p.log(fmt.Sprintf("    judge candidate lookup failed for %s: %v", article.ID, err), logger.ColorRed)
		return
	}
	if len(candidates) == 0 {
		p.log(fmt.Sprintf("    judge: no candidate feeds for %s", article.Title), logger.ColorBlue)
		return
	}

	results, err := p.evaluator.Evaluate(ctx, candidates, article.Title, keywords)
	if err != nil {
		p.log(fmt.Sprintf("    judge failed for %s: %v", article.ID, err), logger.ColorRed)
		return
	}

	for _, r := range results {
		if !r.Passed {
			p.log(fmt.Sprintf("    %s: feed %q (overlap %d, score %d)", r.Decision, r.Feed.Name, r.OverlapCount, r.Score), logger.ColorBlue)
			continue
		}
		if err := p.runTx(ctx, func(q db.Querier) error {
			_, e := p.afCtrl.Create(ctx, q, article.ID, r.Feed.ID)
			return e
		}); err != nil {
			p.log(fmt.Sprintf("    judge: failed to associate %s to feed %s: %v", article.ID, r.Feed.ID, err), logger.ColorRed)
			continue
		}
		if r.Decision == judgement.DecisionAutoAssociated {
			p.log(fmt.Sprintf("    auto-associated into feed %q (overlap %d)", r.Feed.Name, r.OverlapCount), logger.ColorGreen)
		} else {
			p.log(fmt.Sprintf("    judged into feed %q (score %d)", r.Feed.Name, r.Score), logger.ColorGreen)
		}
	}
}

func (p *TreatmentProcessor) alreadyExists(ctx context.Context, urlOriginal string) (bool, error) {
	var exists bool
	err := p.runTx(ctx, func(q db.Querier) error {
		_, err := p.articleCtrl.FindByURLOriginal(ctx, q, urlOriginal)
		if err == nil {
			exists = true
			return nil
		}
		if errors.Is(err, controllers.ErrArticleNotFound) {
			return nil
		}
		return err
	})
	return exists, err
}

// resolveInternalURLs is the url-treatment resolver: given the anchor hrefs found in an article, it
// returns, for the ones whose exact url_original matches an article we already have, the internal
// client URL to point them at. All lookups share one short read transaction. A missing article is not
// an error (the link simply stays external); a real DB error aborts and is surfaced to the caller.
func (p *TreatmentProcessor) resolveInternalURLs(ctx context.Context, hrefs []string) (map[string]string, error) {
	out := make(map[string]string, len(hrefs))
	err := p.runTx(ctx, func(q db.Querier) error {
		for _, href := range hrefs {
			article, err := p.articleCtrl.FindByURLOriginal(ctx, q, href)
			switch {
			case err == nil:
				out[href] = p.clientURL + "/articles/" + article.ID
			case errors.Is(err, controllers.ErrArticleNotFound):
				// Not one of ours: leave the link as-is.
			default:
				return err
			}
		}
		return nil
	})
	return out, err
}

// detectLanguage runs lingua-go over the raw title+content and returns the ISO code, or nil when
// detection is not reliable (persisted as a null language_original).
func (p *TreatmentProcessor) detectLanguage(title, content string) *string {
	code, ok := p.detector.Detect(title, content)
	if !ok {
		return nil
	}
	return &code
}

// persist runs the 0.45 "database operations" block for one treated article, all in a single
// transaction so a mid-way crash never leaves a body referencing outbound rows that don't exist:
//  1. Gravação: store the article (body still holds real URLs).
//  2. Associação + Reorganização: assign an outbound-link id to every distinct <a href>, persist the
//     links, and write the id-form body back. Best-effort — a parse failure keeps the real-URL body
//     and no rows (Resolve leaves non-id hrefs alone), so the rewrite is lost but the article is not.
//  3. Alteração: retro-link older articles that had pointed at this article's external URL, by
//     retargeting their outbound rows to this article's internal (token-form) URL.
//
// A real DB write error at any step rolls the whole thing back (the article is then rediscovered and
// retried next run). Judgement runs afterwards, outside this transaction.
func (p *TreatmentProcessor) persist(ctx context.Context, article DiscoveredArticle, content string, keywords []string, languageOriginal *string) (db.Article, error) {
	var saved db.Article
	err := p.runTx(ctx, func(q db.Querier) error {
		var e error
		saved, e = p.articleCtrl.Create(ctx, q, article.Title, content, article.URLOriginal, article.SourceID, keywords, languageOriginal)
		if e != nil {
			return e
		}

		rewritten, links, aerr := outboundlinks.Assign(saved.Content, p.clientURL, newOutboundID)
		if aerr != nil {
			// Losing the rewrite must not drop the article: keep the real-URL body and no rows.
			p.log(fmt.Sprintf("    outbound assign failed for %s: %v (keeping real-URL body)", saved.ID, aerr), logger.ColorRed)
		} else if len(links) > 0 {
			for _, l := range links {
				if _, e := p.outboundCtrl.Create(ctx, q, l.ID, saved.ID, l.Href); e != nil {
					return e
				}
			}
			if e := p.articleCtrl.UpdateContent(ctx, q, saved.ID, rewritten); e != nil {
				return e
			}
			saved.Content = rewritten
		}

		// Retroactive linking: any older article whose outbound row points at this article's external
		// URL now opens it internally. No-op when nothing matches; the token defers CLIENT_URL to read.
		return p.outboundCtrl.Retarget(ctx, q, saved.UrlOriginal, outboundlinks.InternalHref(saved.ID))
	})
	return saved, err
}

// newOutboundID generates a UUIDv7 for an outbound link (same id written into the body anchor and the
// row). Matches the outboundlinks.Assign newID contract.
func newOutboundID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (p *TreatmentProcessor) log(message string, color logger.Color) {
	if p.verbose {
		logger.Print(message, color)
	}
}
