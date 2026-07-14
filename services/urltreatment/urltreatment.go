// Package urltreatment rewrites in-content links that point to an article we already have, making
// them point to our own client instead of the external source. It is the deterministic step that
// runs right before sanitization in the treatment pipeline: it only touches <a href> anchors, and it
// only rewrites a href when it matches the exact url_original of one of our articles — everything
// else is left untouched (the sanitize step keeps external links).
//
// The package is DB-agnostic: the caller injects a Resolve func that maps the discovered hrefs to
// internal client URLs. That keeps the HTML parsing/rewriting pure and testable, and leaves the
// database access (and its transaction) to the caller, per the project's transaction convention.
package urltreatment

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/nathanap/news-feed-backend/logger"
)

// Resolve receives the unique, non-empty hrefs found in the content and returns a map from href to
// internal client URL, containing only the hrefs that match one of our articles. Hrefs absent from
// the returned map are left unchanged.
type Resolve func(ctx context.Context, hrefs []string) (map[string]string, error)

// Treat parses the article body as an HTML fragment, finds every <a href>, asks resolve which of
// those hrefs point to an article we already have, and rewrites those anchors to the internal client
// URL. It is best-effort: on a parse, resolve or render error it returns the ORIGINAL content
// unchanged (plus the error, for the caller to log) — losing the rewrite must never drop the article.
func Treat(ctx context.Context, content string, resolve Resolve, verbose bool) (string, error) {
	nodes, err := parseFragment(content)
	if err != nil {
		return content, err
	}

	hrefs := collectHrefs(nodes)
	if verbose {
		logger.Print(fmt.Sprintf("@@@ URL TREATMENT START - %d anchor href(s) found @@@", len(hrefs)), logger.ColorCyan)
		for _, h := range hrefs {
			logger.Print("    found: "+h, logger.ColorBlue)
		}
	}
	if len(hrefs) == 0 {
		return content, nil
	}

	mapping, err := resolve(ctx, hrefs)
	if err != nil {
		return content, err
	}
	if len(mapping) == 0 {
		if verbose {
			logger.Print(fmt.Sprintf("@@@ URL TREATMENT END - 0 of %d link(s) are ours; nothing rewritten @@@", len(hrefs)), logger.ColorGreen)
		}
		return content, nil
	}

	rewritten := 0
	for _, n := range nodes {
		rewritten += rewriteAnchors(n, mapping)
	}
	if verbose {
		for external, internal := range mapping {
			logger.Print(fmt.Sprintf("    rewrote: %s -> %s", external, internal), logger.ColorGreen)
		}
	}

	out, err := renderFragment(nodes)
	if err != nil {
		return content, err
	}

	if verbose {
		logger.Print(fmt.Sprintf("@@@ URL TREATMENT END - %d internal link(s) rewritten @@@", rewritten), logger.ColorGreen)
	}
	return out, nil
}

// parseFragment parses content as the inner HTML of a <body>, so no <html>/<head>/<body> wrappers are
// injected into the result (unlike html.Parse).
func parseFragment(content string) ([]*html.Node, error) {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	return html.ParseFragment(strings.NewReader(content), body)
}

// collectHrefs returns the unique, non-empty href values of every <a> in the fragment, in first-seen
// order (so resolve is asked about each distinct href only once).
func collectHrefs(nodes []*html.Node) []string {
	seen := make(map[string]struct{})
	var hrefs []string
	for _, n := range nodes {
		walk(n, func(a *html.Node) {
			href, ok := getAttr(a, "href")
			if !ok || href == "" {
				return
			}
			if _, dup := seen[href]; dup {
				return
			}
			seen[href] = struct{}{}
			hrefs = append(hrefs, href)
		})
	}
	return hrefs
}

// rewriteAnchors sets the href of every <a> whose current href is in mapping, returning how many were
// rewritten.
func rewriteAnchors(root *html.Node, mapping map[string]string) int {
	count := 0
	walk(root, func(a *html.Node) {
		for i := range a.Attr {
			if a.Attr[i].Key != "href" {
				continue
			}
			if internal, ok := mapping[a.Attr[i].Val]; ok {
				a.Attr[i].Val = internal
				count++
			}
		}
	})
	return count
}

// walk invokes fn for every <a> element in the subtree rooted at n.
func walk(n *html.Node, fn func(anchor *html.Node)) {
	if n.Type == html.ElementNode && n.DataAtom == atom.A {
		fn(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func getAttr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func renderFragment(nodes []*html.Node) (string, error) {
	var buf bytes.Buffer
	for _, n := range nodes {
		if err := html.Render(&buf, n); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}
