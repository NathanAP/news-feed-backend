package discovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// TreatmentProcessor is the real discovery sink: for each discovered article it deduplicates by
// url_original, runs the two-step AI treatment (clean content, then keywords over the cleaned
// content), and persists the result. AI calls happen outside any transaction. A failure on one
// article (AI error, persistence error) is logged and skipped — since nothing is persisted for
// it, the article is simply rediscovered and retried on the next run.
type TreatmentProcessor struct {
	runTx       controllers.TransactionRunner
	articleCtrl controllers.ArticleControllerInterface
	treater     ai.Treater
	keyworder   ai.Keyworder
	verbose     bool
}

func NewTreatmentProcessor(
	runTx controllers.TransactionRunner,
	articleCtrl controllers.ArticleControllerInterface,
	treater ai.Treater,
	keyworder ai.Keyworder,
	verbose bool,
) *TreatmentProcessor {
	return &TreatmentProcessor{
		runTx:       runTx,
		articleCtrl: articleCtrl,
		treater:     treater,
		keyworder:   keyworder,
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

		if err := p.persist(ctx, article, treated, keywords); err != nil {
			if errors.Is(err, controllers.ErrArticleAlreadyExists) {
				continue // created concurrently between the dedup check and the insert
			}
			p.log(fmt.Sprintf("    persist failed for %s: %v", article.URLOriginal, err), logger.ColorRed)
			continue
		}

		p.log(fmt.Sprintf("    saved: %s (%s)", article.Title, article.URLOriginal), logger.ColorGreen)
	}
	return nil
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

func (p *TreatmentProcessor) persist(ctx context.Context, article DiscoveredArticle, content string, keywords []string) error {
	return p.runTx(ctx, func(q db.Querier) error {
		_, err := p.articleCtrl.Create(ctx, q, article.Title, content, article.URLOriginal, article.SourceID, keywords)
		return err
	})
}

func (p *TreatmentProcessor) log(message string, color logger.Color) {
	if p.verbose {
		logger.Print(message, color)
	}
}
