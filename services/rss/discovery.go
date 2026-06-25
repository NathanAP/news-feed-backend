package rss

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/mmcdole/gofeed"
)

var (
	// matches href in <link rel="alternate" type="application/rss+xml" href="...">
	// also matches atom+xml and handles attribute order variations
	feedLinkPattern = regexp.MustCompile(`(?i)<link[^>]+type="application/(rss|atom)\+xml"[^>]*href="([^"]+)"`)
	feedHrefPattern = regexp.MustCompile(`(?i)<link[^>]+href="([^"]+)"[^>]*type="application/(rss|atom)\+xml"`)

	commonPaths = []string{
		"/rss",
		"/feed",
		"/rss.xml",
		"/feed.xml",
		"/atom.xml",
		"/rss/",
		"/feed/",
		"/?feed=rss2",
		"/feeds/posts/default",
		"/blog/rss",
		"/blog/feed",
		"/news/rss",
		"/news/feed",
	}
)

// Discover fetches baseURL, parses it for RSS/Atom link tags, then tries common
// feed paths as a fallback. Each candidate is validated with gofeed. Returns only
// URLs that parse as a valid RSS or Atom feed. Returns an empty slice when none
// are found — callers should treat that as a normal result, not an error.
func Discover(ctx context.Context, client *http.Client, baseURL string) ([]string, error) {
	candidates := discoverFromHTML(ctx, client, baseURL)

	if len(candidates) == 0 {
		candidates = discoverFromCommonPaths(ctx, client, baseURL)
	}

	return validateFeeds(ctx, client, candidates), nil
}

// discoverFromHTML fetches the page HTML and extracts <link rel="alternate"> feed URLs.
func discoverFromHTML(ctx context.Context, client *http.Client, baseURL string) []string {
	body, err := fetchBody(ctx, client, baseURL)
	if err != nil {
		return nil
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	var found []string
	seen := map[string]struct{}{}

	for _, match := range feedLinkPattern.FindAllStringSubmatch(body, -1) {
		if href := resolveHref(match[2], base); href != "" {
			if _, ok := seen[href]; !ok {
				seen[href] = struct{}{}
				found = append(found, href)
			}
		}
	}
	for _, match := range feedHrefPattern.FindAllStringSubmatch(body, -1) {
		if href := resolveHref(match[1], base); href != "" {
			if _, ok := seen[href]; !ok {
				seen[href] = struct{}{}
				found = append(found, href)
			}
		}
	}

	return found
}

// discoverFromCommonPaths concurrently probes well-known feed paths under baseURL.
func discoverFromCommonPaths(ctx context.Context, client *http.Client, baseURL string) []string {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var found []string

	for _, path := range commonPaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			candidate := base.Scheme + "://" + base.Host + p
			if body, err := fetchBody(ctx, client, candidate); err == nil && body != "" {
				mu.Lock()
				found = append(found, candidate)
				mu.Unlock()
			}
		}(path)
	}

	wg.Wait()
	return found
}

// validateFeeds returns only candidates that gofeed can successfully parse.
func validateFeeds(ctx context.Context, client *http.Client, candidates []string) []string {
	var valid []string
	parser := gofeed.NewParser()

	for _, candidate := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		_, parseErr := parser.Parse(resp.Body)
		resp.Body.Close()
		if parseErr == nil {
			valid = append(valid, candidate)
		}
	}

	return valid
}

func fetchBody(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func resolveHref(href string, base *url.URL) string {
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref)
	scheme := resolved.Scheme
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return resolved.String()
}
