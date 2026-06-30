package sources

import (
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/discovery"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// SourceDiscovery is a dry-run of the discovery CRON scoped to a single source: it fetches the
// feed and returns what *would* be discovered, without persisting anything or advancing the
// watermark. The optional `last_article_discovery_at` query overrides the lower bound, so the
// caller can test without waiting for a genuinely new article. Open for now (admin-future).
func SourceDiscovery(ctrl controllers.SourceControllerInterface, systemCtrl controllers.SystemControllerInterface, runTx controllers.TransactionRunner, httpClient *http.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		id := c.Params("id")

		var sinceOverride *time.Time
		if raw := c.Query("last_article_discovery_at"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "last_article_discovery_at must be an RFC3339 date (UTC)",
				})
			}
			utc := parsed.UTC()
			sinceOverride = &utc
		}

		var (
			source db.Source
			since  time.Time
		)
		now := time.Now().UTC()
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			source, err = ctrl.FindByID(c.Context(), q, id)
			if err != nil {
				return err
			}
			if sinceOverride != nil {
				since = *sinceOverride
				return nil
			}
			system, err := systemCtrl.Get(c.Context(), q)
			if err != nil {
				return err
			}
			since = discovery.EffectiveSince(system.LastArticleDiscoveryAt, now)
			return nil
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

		return c.JSON(schemas.SourceDiscoveryResponse{Articles: toDiscoveredArticleResponses(items)})
	}
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
