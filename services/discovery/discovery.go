package discovery

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/sethvargo/go-retry"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// DiscoveredArticle is a raw item pulled from a source's RSS feed, before any AI treatment or
// persistence. It is what the discovery pipeline hands to a Processor.
type DiscoveredArticle struct {
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	URLOriginal string     `json:"url_original"`
	PublishedAt *time.Time `json:"published_at"`
	SourceID    string     `json:"source_id"`
}

// Processor consumes the articles found by a discovery run. The deduplication-by-url_original,
// AI treatment and persistence all live behind this seam (see TreatmentProcessor).
type Processor interface {
	Process(ctx context.Context, articles []DiscoveredArticle) error
}

// DiscoverFromSource fetches a source's RSS feed and returns its items. Deduplication is done
// downstream by url_original (the reliable identity), so by default every current feed item is
// returned. The optional `since` lower bound is a convenience for the dry-run test endpoint: when
// non-zero, items published on or before it are dropped; items without a publication date are
// always kept. A feed that is unreachable or unparseable returns an error so callers can isolate
// it — one bad source must never abort a whole run.
func DiscoverFromSource(ctx context.Context, client *http.Client, source db.Source, since time.Time) ([]DiscoveredArticle, error) {
	feed, err := fetchFeed(ctx, client, source.UrlRss)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed %q: %w", source.UrlRss, err)
	}

	discovered := make([]DiscoveredArticle, 0, len(feed.Items))
	for _, item := range feed.Items {
		if item == nil {
			continue
		}

		var publishedAt *time.Time
		if item.PublishedParsed != nil {
			utc := item.PublishedParsed.UTC()
			publishedAt = &utc
			if !since.IsZero() && !utc.After(since) {
				continue // older than (or equal to) the requested lower bound
			}
		}

		discovered = append(discovered, DiscoveredArticle{
			Title:       item.Title,
			Content:     itemContent(item),
			URLOriginal: item.Link,
			PublishedAt: publishedAt,
			SourceID:    source.ID,
		})
	}

	return discovered, nil
}

func itemContent(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
}

// fetchFeed retrieves and parses an RSS/Atom feed, retrying transient failures with exponential
// backoff so a flaky source does not fail the run on the first hiccup.
func fetchFeed(ctx context.Context, client *http.Client, feedURL string) (*gofeed.Feed, error) {
	parser := gofeed.NewParser()

	var feed *gofeed.Feed
	backoff := retry.WithMaxRetries(3, retry.NewExponential(200*time.Millisecond))
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
		if err != nil {
			return err // malformed URL — not retryable
		}

		resp, err := client.Do(req)
		if err != nil {
			return retry.RetryableError(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return retry.RetryableError(fmt.Errorf("unexpected status %d", resp.StatusCode))
		}

		parsed, err := parser.Parse(resp.Body)
		if err != nil {
			return retry.RetryableError(err)
		}

		feed = parsed
		return nil
	})
	if err != nil {
		return nil, err
	}
	return feed, nil
}
