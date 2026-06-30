package discovery

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/sethvargo/go-retry"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// DiscoveredArticle is a raw item pulled from a source's RSS feed, before any AI treatment or
// persistence (both land in 0.20). It is what the discovery pipeline hands to a Processor.
type DiscoveredArticle struct {
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	URLOriginal string     `json:"url_original"`
	PublishedAt *time.Time `json:"published_at"`
	SourceID    string     `json:"source_id"`
}

// Processor consumes the articles found by a discovery run. In 0.19 nothing is persisted, so the
// wired implementation is a no-op; in 0.20 it will run treatment, judgment and persistence. This
// interface is the evolution seam the roadmap asks to leave prepared.
type Processor interface {
	Process(ctx context.Context, articles []DiscoveredArticle) error
}

// NoopProcessor is the 0.19 processor: discovery only fetches and reports, it does not write
// anything to the database.
type NoopProcessor struct{}

func NewNoopProcessor() *NoopProcessor {
	return &NoopProcessor{}
}

func (NoopProcessor) Process(_ context.Context, _ []DiscoveredArticle) error {
	return nil
}

// EffectiveSince returns the lower bound used to decide what counts as a "new" article. When the
// watermark is unset (the first ever run) it falls back to `now`, so the first run does not flood
// the pipeline with each feed's entire current backlog (per PROJECT.md, "Descobrindo uma notícia").
func EffectiveSince(watermark sql.NullTime, now time.Time) time.Time {
	if watermark.Valid {
		return watermark.Time.UTC()
	}
	return now.UTC()
}

// DiscoverFromSource fetches a source's RSS feed and returns the items published after `since`.
// Items without a publication date are always included — they cannot be placed on the timeline,
// and deduplication by url_original (added with persistence in 0.20) is the real safeguard against
// duplicates. A feed that is unreachable or unparseable returns an error so callers can isolate
// it; one bad source must never abort a whole run.
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
			if !utc.After(since) {
				continue // older than (or equal to) the watermark — not new
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
