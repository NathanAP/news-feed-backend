package sources

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/rss"
)

// rssDiscoveryBudget caps one discovery attempt end to end. Generous enough for a slow but real site
// (the probes run concurrently), short enough that a deliberately unresponsive host cannot park the
// handler. Discovery is best-effort by design, so expiring simply yields whatever was found.
const rssDiscoveryBudget = 45 * time.Second

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

		// The client's timeout bounds each request; this bounds the whole fan-out. rss.Discover issues
		// ~15 requests for one call (the page, 13 concurrent well-known paths, then one per candidate),
		// so per-request timeouts alone still allow a caller-chosen slow host to hold the handler for
		// several multiples of that. c.Context() carries no deadline of its own.
		ctx, cancel := context.WithTimeout(c.Context(), rssDiscoveryBudget)
		defer cancel()

		feeds, _ := rss.Discover(ctx, httpClient, rawURL)
		if feeds == nil {
			feeds = []string{}
		}

		return c.JSON(schemas.RSSDiscoveryResponse{Feeds: feeds})
	}
}
