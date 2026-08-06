package sources

import (
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/discovery"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// SourceArticleDiscovery is a dry-run of the discovery CRON scoped to a single source: it runs the
// exact same steps the CRON would (fetch the feed, deduplicate by url_original) and returns what
// *would* go forward to treatment — without persisting anything or advancing the watermark. The
// optional `last_article_discovery_at` query adds a date lower bound so the caller can test without
// waiting for a genuinely new article. Administrator-only since 0.40.
func SourceArticleDiscovery(sourceCtrl controllers.SourceControllerInterface, articleCtrl controllers.ArticleControllerInterface, runTx controllers.TransactionRunner, httpClient *http.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		id := c.Params("id")

		var since time.Time
		if raw := c.Query("last_article_discovery_at"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "last_article_discovery_at must be an RFC3339 date (UTC)",
				})
			}
			since = parsed.UTC()
		}

		var source db.Source
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			source, err = sourceCtrl.FindByID(c.Context(), q, id)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrSourceNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "source not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load source"})
		}

		items, err := discovery.DiscoverFromSource(c.Context(), httpClient, source, since)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to discover articles"})
		}

		// Deduplicate by url_original exactly like the CRON would — read-only, nothing is written.
		fresh, err := filterNewByURLOriginal(c, articleCtrl, runTx, items)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to deduplicate articles"})
		}

		return c.JSON(schemas.SourceDiscoveryResponse{Articles: toDiscoveredArticleResponses(fresh)})
	}
}

// filterNewByURLOriginal drops items whose url_original already exists as an active article.
func filterNewByURLOriginal(c fiber.Ctx, articleCtrl controllers.ArticleControllerInterface, runTx controllers.TransactionRunner, items []discovery.DiscoveredArticle) ([]discovery.DiscoveredArticle, error) {
	fresh := make([]discovery.DiscoveredArticle, 0, len(items))
	err := runTx(c.Context(), func(q db.Querier) error {
		for _, item := range items {
			_, err := articleCtrl.FindByURLOriginal(c.Context(), q, item.URLOriginal)
			if err == nil {
				continue // already exists
			}
			if !errors.Is(err, controllers.ErrArticleNotFound) {
				return err
			}
			fresh = append(fresh, item)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

func toDiscoveredArticleResponses(items []discovery.DiscoveredArticle) []schemas.DiscoveredArticleResponse {
	out := make([]schemas.DiscoveredArticleResponse, 0, len(items))
	for _, item := range items {
		out = append(out, schemas.DiscoveredArticleResponse{
			Title:       item.Title,
			Content:     item.Content,
			URLOriginal: item.URLOriginal,
			PublishedAt: item.PublishedAt,
			SourceID:    item.SourceID,
		})
	}
	return out
}
