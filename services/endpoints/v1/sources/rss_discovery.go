package sources

import (
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/rss"
)

func RSSDiscovery(httpClient *http.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		rawURL := c.Query("url")
		if rawURL == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url query parameter is required"})
		}

		parsed, err := url.ParseRequestURI(rawURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url must be a valid http or https URL"})
		}

		feeds, _ := rss.Discover(c.Context(), httpClient, rawURL)
		if feeds == nil {
			feeds = []string{}
		}

		return c.JSON(schemas.RSSDiscoveryResponse{Feeds: feeds})
	}
}
