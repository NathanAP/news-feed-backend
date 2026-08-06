package articles

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/embedtreatment"
	"github.com/nathanap/news-feed-backend/services/langdetect"
	"github.com/nathanap/news-feed-backend/services/sanitize"
	"github.com/nathanap/news-feed-backend/services/urltreatment"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// TreatArticle is a dry-run of the treatment step: it takes raw article data (as produced by
// discovery), detects the language, rewrites in-content links to articles we already have (url
// treatment, read-only lookups) and sanitizes the raw body to the safe-HTML whitelist (both
// deterministic, no AI) then names keywords, and returns the result plus per-step timings. The
// keyword-naming backend is chosen by the configured default mode, or overridden per call via the
// body's `keywords_mode` (local | groq | gemini) so backends can be benchmarked from Bruno without
// restarting. It does NOT persist anything, but reads the DB (url treatment) and calls the keyword AI
// for real (consumes quota).
//
// Reachable by any authenticated user, but the real guard is that main.go registers this route ONLY
// when ENVIRONMENT=development, so it does not exist at all in staging or production. Making it
// administrator-only would be belt-and-braces, not the thing keeping it safe.
//
// It stops before the 0.45 "database operations" block (outbound-link assignment / retargeting), which
// is inherently a write step and cannot run in a dry-run: the returned content therefore carries real
// URLs, not the id form the stored body would have. That block is exercised by the discovery
// integration tests instead.
func TreatArticle(keyworders map[string]ai.Keyworder, defaultMode string, detector langdetect.Detector, articleCtrl controllers.ArticleControllerInterface, runTx controllers.TransactionRunner, clientURL string, urlVerbose bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		var req schemas.TreatArticleRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if strings.TrimSpace(req.Article.Title) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "article.title is required"})
		}
		if strings.TrimSpace(req.Article.Content) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "article.content is required"})
		}

		// Language detection mirrors the CRON step: lingua-go over the raw content, empty when
		// detection is not reliable. Not AI, so it does not consume quota.
		languageOriginal, _ := detector.Detect(req.Article.Title, req.Article.Content)

		mode := defaultMode
		if req.KeywordsMode != "" {
			mode = req.KeywordsMode
		}
		keyworder, ok := keyworders[mode]
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("unknown keywords_mode %q (available: %s)", mode, availableModes(keyworders)),
			})
		}

		// Body treatment is deterministic (no AI): url treatment (rewrite internal links, read-only) +
		// embed treatment (script embeds -> links) then sanitize to the safe whitelist. Mirrors the
		// CRON pipeline exactly.
		treatStart := time.Now()
		body := req.Article.Content
		if clientURL != "" {
			resolve := internalURLResolver(articleCtrl, runTx, clientURL)
			if out, terr := urltreatment.Treat(c.Context(), req.Article.Content, resolve, urlVerbose); terr != nil {
				logger.Log(fmt.Sprintf("url treatment failed: %v (using original content)", terr), logger.ColorRed)
			} else {
				body = out
			}
		}
		if out, eerr := embedtreatment.Treat(body, clientURL, urlVerbose); eerr != nil {
			logger.Log(fmt.Sprintf("embed treatment failed: %v (using original content)", eerr), logger.ColorRed)
		} else {
			body = out
		}
		treated := sanitize.Sanitize(body)
		treatmentMs := time.Since(treatStart).Milliseconds()

		kwStart := time.Now()
		// Keywords run over the plain text (markup stripped), mirroring the CRON.
		keywords, err := keyworder.Keywords(c.Context(), req.Article.Title, sanitize.PlainText(treated))
		keywordsMs := time.Since(kwStart).Milliseconds()
		if err != nil {
			logger.Log(fmt.Sprintf("keywords failed: %v", err), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to assign keywords"})
		}

		return c.JSON(schemas.ArticleTreatmentResponse{
			Content:          treated,
			Keywords:         keywords,
			KeywordsMode:     mode,
			LanguageOriginal: languageOriginal,
			TreatmentMs:      treatmentMs,
			KeywordsMs:       keywordsMs,
		})
	}
}

// internalURLResolver builds the url-treatment resolver used by the dry-run: it maps discovered
// hrefs to internal client URLs for the ones whose exact url_original matches an existing article.
// All lookups share one short read transaction; a missing article is not an error.
func internalURLResolver(articleCtrl controllers.ArticleControllerInterface, runTx controllers.TransactionRunner, clientURL string) urltreatment.Resolve {
	return func(ctx context.Context, hrefs []string) (map[string]string, error) {
		out := make(map[string]string, len(hrefs))
		err := runTx(ctx, func(q db.Querier) error {
			for _, href := range hrefs {
				article, err := articleCtrl.FindByURLOriginal(ctx, q, href)
				switch {
				case err == nil:
					out[href] = clientURL + "/articles/" + article.ID
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
}

// availableModes lists the mode keys of an AI-backend map (keyworders, judgers), sorted, for error
// messages. Generic so it serves every per-mode map without duplication.
func availableModes[T any](backends map[string]T) string {
	modes := make([]string, 0, len(backends))
	for m := range backends {
		modes = append(modes, m)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}
