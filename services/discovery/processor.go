package discovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/judgment"
	"github.com/nathanap/news-feed-backend/services/langdetect"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// TreatmentProcessor is the real discovery sink: for each discovered article it deduplicates by
// url_original, runs the two-step AI treatment (clean content, then keywords over the cleaned
// content), persists the result, and finally judges the article against the user feeds — writing an
// articles_feeds association for every feed that clears the threshold. AI calls happen outside any
// transaction. A failure on one article (AI error, persistence error) is logged and skipped — since
// nothing is persisted for it, the article is simply rediscovered and retried on the next run.
// Judgement is best-effort and non-retroactive: once the article is persisted, a judgement failure
// is logged and skipped, and the article is not re-judged on later runs (dedup skips it).
type TreatmentProcessor struct {
	runTx       controllers.TransactionRunner
	articleCtrl controllers.ArticleControllerInterface
	feedCtrl    controllers.FeedControllerInterface
	afCtrl      controllers.ArticleFeedControllerInterface
	detector    langdetect.Detector
	treater     ai.Treater
	keyworder   ai.Keyworder
	evaluator   *judgment.Evaluator
	verbose     bool
}

func NewTreatmentProcessor(
	runTx controllers.TransactionRunner,
	articleCtrl controllers.ArticleControllerInterface,
	feedCtrl controllers.FeedControllerInterface,
	afCtrl controllers.ArticleFeedControllerInterface,
	detector langdetect.Detector,
	treater ai.Treater,
	keyworder ai.Keyworder,
	evaluator *judgment.Evaluator,
	verbose bool,
) *TreatmentProcessor {
	return &TreatmentProcessor{
		runTx:       runTx,
		articleCtrl: articleCtrl,
		feedCtrl:    feedCtrl,
		afCtrl:      afCtrl,
		detector:    detector,
		treater:     treater,
		keyworder:   keyworder,
		evaluator:   evaluator,
		verbose:     verbose,
	}
}

var _ Processor = (*TreatmentProcessor)(nil)

func (p *TreatmentProcessor) Process(ctx context.Context, articles []DiscoveredArticle) error {
	for _, article := range articles {
		exists, err := p.alreadyExists(ctx, article.URLOriginal)
		if err != nil {
			p.log(fmt.Sprintf("    dedup check failed for %s: %v", article.URLOriginal, err), logger.ColorRed)
			continue
		}
		if exists {
			p.log(fmt.Sprintf("    skip (already exists): %s", article.URLOriginal), logger.ColorBlue)
			continue
		}

		treated, err := p.treater.Treat(ctx, article.Title, article.Content)
		if err != nil {
			p.log(fmt.Sprintf("    treat failed for %s: %v", article.URLOriginal, err), logger.ColorRed)
			continue
		}

		keywords, err := p.keyworder.Keywords(ctx, article.Title, treated)
		if err != nil {
			p.log(fmt.Sprintf("    keywords failed for %s: %v", article.URLOriginal, err), logger.ColorRed)
			continue
		}

		// Detect the original language from the raw (untreated) text. This is a lingua-go call, not
		// AI: deterministic and offline. A failed detection is not fatal — the article is stored with
		// a null language_original (it just cannot be translated later).
		languageOriginal := p.detectLanguage(article.Title, article.Content)

		saved, err := p.persist(ctx, article, treated, keywords, languageOriginal)
		if err != nil {
			if errors.Is(err, controllers.ErrArticleAlreadyExists) {
				continue // created concurrently between the dedup check and the insert
			}
			p.log(fmt.Sprintf("    persist failed for %s: %v", article.URLOriginal, err), logger.ColorRed)
			continue
		}

		p.log(fmt.Sprintf("    saved: %s (%s)", article.Title, article.URLOriginal), logger.ColorGreen)

		// Judgement is best-effort: the article is already persisted, so a failure here must not
		// abort the run — it is logged and the next article proceeds.
		p.judge(ctx, saved, keywords)
	}
	return nil
}

// judge runs the two judgement layers for a freshly persisted article and writes an articles_feeds
// association for every feed that clears the threshold. Layer 1 (candidate feeds) and the winner
// inserts run in their own transactions; the AI scoring (layer 2) runs in between, outside any
// transaction.
func (p *TreatmentProcessor) judge(ctx context.Context, article db.Article, keywords []string) {
	var candidates []db.Feed
	if err := p.runTx(ctx, func(q db.Querier) error {
		var e error
		candidates, e = p.feedCtrl.FindCandidatesByKeywords(ctx, q, keywords)
		return e
	}); err != nil {
		p.log(fmt.Sprintf("    judge candidate lookup failed for %s: %v", article.ID, err), logger.ColorRed)
		return
	}
	if len(candidates) == 0 {
		p.log(fmt.Sprintf("    judge: no candidate feeds for %s", article.Title), logger.ColorBlue)
		return
	}

	results, err := p.evaluator.Evaluate(ctx, candidates, article.Title, keywords, article.Content)
	if err != nil {
		p.log(fmt.Sprintf("    judge failed for %s: %v", article.ID, err), logger.ColorRed)
		return
	}

	for _, r := range results {
		if !r.Passed {
			continue
		}
		if err := p.runTx(ctx, func(q db.Querier) error {
			_, e := p.afCtrl.Create(ctx, q, article.ID, r.Feed.ID)
			return e
		}); err != nil {
			p.log(fmt.Sprintf("    judge: failed to associate %s to feed %s: %v", article.ID, r.Feed.ID, err), logger.ColorRed)
			continue
		}
		p.log(fmt.Sprintf("    judged into feed %q (score %d)", r.Feed.Name, r.Score), logger.ColorGreen)
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

// detectLanguage runs lingua-go over the raw title+content and returns the ISO code, or nil when
// detection is not reliable (persisted as a null language_original).
func (p *TreatmentProcessor) detectLanguage(title, content string) *string {
	code, ok := p.detector.Detect(title, content)
	if !ok {
		return nil
	}
	return &code
}

func (p *TreatmentProcessor) persist(ctx context.Context, article DiscoveredArticle, content string, keywords []string, languageOriginal *string) (db.Article, error) {
	var saved db.Article
	err := p.runTx(ctx, func(q db.Querier) error {
		var e error
		saved, e = p.articleCtrl.Create(ctx, q, article.Title, content, article.URLOriginal, article.SourceID, keywords, languageOriginal)
		return e
	})
	return saved, err
}

func (p *TreatmentProcessor) log(message string, color logger.Color) {
	if p.verbose {
		logger.Print(message, color)
	}
}
