package external

import (
	"io"
	"net/http"
	"strings"
)

// MockRSSTransport implements http.RoundTripper for RSS/HTTP client mocking in tests.
// Responses keyed by URL; missing URLs return 404.
type MockRSSTransport struct {
	Responses map[string]MockRSSResponse
}

type MockRSSResponse struct {
	StatusCode int
	Body       string
	Err        error
}

func (m *MockRSSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	urlStr := req.URL.String()
	if mock, ok := m.Responses[urlStr]; ok {
		if mock.Err != nil {
			return nil, mock.Err
		}
		return &http.Response{
			StatusCode: mock.StatusCode,
			Body:       io.NopCloser(strings.NewReader(mock.Body)),
			Header:     make(http.Header),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func NewMockRSSClient(responses map[string]MockRSSResponse) *http.Client {
	return &http.Client{Transport: &MockRSSTransport{Responses: responses}}
}

// SampleRSSFeed is a minimal valid RSS 2.0 document for test responses.
const SampleRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>A test RSS feed</description>
    <item>
      <title>Test Item</title>
      <link>https://example.com/item-1</link>
      <description>Test item description</description>
    </item>
  </channel>
</rss>`

// SampleRSSFeedDated is an RSS 2.0 document with dated items plus one undated item, used to test
// the discovery "new since" filter. Dates: old = 2025-01-06, fresh = 2025-01-10.
const SampleRSSFeedDated = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Dated Feed</title>
    <link>https://example.com</link>
    <description>A dated test RSS feed</description>
    <item>
      <title>Old News</title>
      <link>https://example.com/old</link>
      <description>Old item</description>
      <pubDate>Mon, 06 Jan 2025 08:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Fresh News</title>
      <link>https://example.com/fresh</link>
      <description>Fresh item</description>
      <pubDate>Fri, 10 Jan 2025 08:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Undated News</title>
      <link>https://example.com/undated</link>
      <description>Undated item</description>
    </item>
  </channel>
</rss>`

// SampleHTMLWithRSSLink is an HTML page that declares a feed via <link rel="alternate">.
const SampleHTMLWithRSSLink = `<!DOCTYPE html>
<html>
<head>
  <title>Test Page</title>
  <link rel="alternate" type="application/rss+xml" title="Test RSS" href="/rss.xml">
</head>
<body><p>Hello World</p></body>
</html>`

// SampleHTMLWithAtomLink declares an Atom feed in the HTML head.
const SampleHTMLWithAtomLink = `<!DOCTYPE html>
<html>
<head>
  <title>Test Page</title>
  <link rel="alternate" type="application/atom+xml" title="Test Atom" href="/atom.xml">
</head>
<body><p>Hello World</p></body>
</html>`

// SampleHTMLNoFeed is a plain HTML page with no feed declarations.
const SampleHTMLNoFeed = `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body><p>Hello World</p></body>
</html>`
