// Package outboundlinks is the two-way bridge between an article body and the article_outbound_links
// table (0.45). It has no database access of its own: it only parses/rewrites the HTML body, and the
// caller owns the persistence and its transaction (same split as services/urltreatment).
//
// Write side (Assign): during treatment, after the body has been url-treated, embed-treated and
// sanitized, every distinct <a href> is assigned a fresh outbound-link id and the anchor is rewritten
// to carry that id instead of the URL. The URLs to persist come back as []Link.
//
// Read side (Resolve): whenever an article is served, the ids in the body are swapped back to the real
// hrefs (looked up in the table), expanding the {CLIENT_URL} token to the configured client URL.
//
// The stored body therefore never contains a real URL for our anchors — only an id — which is exactly
// what lets a later article retro-link an older one by UPDATEing a row instead of the frozen body.
//
// INVARIANT: an outbound id and the {CLIENT_URL} token are NOT valid URLs, so a body in the id form
// must never pass through bluemonday (it would strip the href). Assign runs after sanitize; Resolve
// runs before any re-sanitization (e.g. the translate path sanitizes the model output, so it must
// Resolve first). Both hold in the current call sites.
package outboundlinks

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// clientURLToken is the literal placeholder stored in place of the configured CLIENT_URL inside an
// internal link's href. Storing the token (not the expanded URL) means a CLIENT_URL change is an env
// change, not a data migration — Resolve expands it at read time.
const clientURLToken = "{CLIENT_URL}"

// InternalHref returns the token-form href stored for a link that points at one of our articles:
// `{CLIENT_URL}/articles/{id}`. The retroactive-retarget step uses it so the value it writes matches
// exactly what Assign stores for an internal link (and what Resolve knows how to expand).
func InternalHref(articleID string) string {
	return clientURLToken + "/articles/" + articleID
}

// Link is one anchor target extracted from a body during Assign: the outbound-link id assigned to it
// and the href to persist in the row. Href is the token form `{CLIENT_URL}/articles/{id}` for an
// internal link, or the literal URL for an external one (mailto included — harmless, it never matches
// a url_original and Resolve restores it verbatim).
type Link struct {
	ID   string
	Href string
}

// Assign finds every distinct <a href> in content, assigns each a fresh id (via newID) and rewrites
// the anchors to carry that id instead of the URL. It returns the rewritten body and the links to
// persist (one per distinct href — identical hrefs share an id, so retro-linking fixes them together).
// Internal hrefs (prefixed by clientURL) are normalized to the token form for storage; everything else
// is stored literally. Best-effort: on a parse/render error it returns the original content and a nil
// slice, so losing the rewrite never drops the article.
func Assign(content, clientURL string, newID func() (string, error)) (string, []Link, error) {
	nodes, err := parseFragment(content)
	if err != nil {
		return content, nil, err
	}

	hrefs := collectDistinctHrefs(nodes)
	if len(hrefs) == 0 {
		return content, nil, nil
	}

	idByHref := make(map[string]string, len(hrefs))
	links := make([]Link, 0, len(hrefs))
	for _, href := range hrefs {
		id, err := newID()
		if err != nil {
			return content, nil, err
		}
		idByHref[href] = id
		links = append(links, Link{ID: id, Href: toStored(href, clientURL)})
	}

	for _, n := range nodes {
		rewriteAnchors(n, func(href string) (string, bool) {
			id, ok := idByHref[href]
			return id, ok
		})
	}

	out, err := renderFragment(nodes)
	if err != nil {
		return content, nil, err
	}
	return out, links, nil
}

// Resolve swaps every <a href="{outbound-id}"> in content back to the real href from byID (id -> stored
// href), expanding the {CLIENT_URL} token to clientURL. Anchors whose href is not a known id are left
// untouched — legacy bodies that still hold real URLs, or ids missing from the map, pass through.
// Best-effort: on a parse/render error it returns content unchanged.
func Resolve(content string, byID map[string]string, clientURL string) string {
	if len(byID) == 0 {
		return content
	}
	nodes, err := parseFragment(content)
	if err != nil {
		return content
	}

	changed := false
	for _, n := range nodes {
		rewriteAnchors(n, func(id string) (string, bool) {
			stored, ok := byID[id]
			if !ok {
				return "", false
			}
			changed = true
			return expand(stored, clientURL), true
		})
	}
	if !changed {
		return content
	}

	out, err := renderFragment(nodes)
	if err != nil {
		return content
	}
	return out
}

// toStored normalizes an internal href to the token form for storage. A href under the configured
// clientURL becomes `{CLIENT_URL}` + the remaining path; everything else (external, mailto) is kept
// literally. clientURL is expected without a trailing slash (main trims it).
func toStored(href, clientURL string) string {
	if clientURL != "" && strings.HasPrefix(href, clientURL) {
		return clientURLToken + href[len(clientURL):]
	}
	return href
}

// expand is the inverse of toStored's token step: it puts the configured clientURL back in place of
// the {CLIENT_URL} token. An empty clientURL yields a root-relative internal link.
func expand(stored, clientURL string) string {
	return strings.ReplaceAll(stored, clientURLToken, clientURL)
}

// parseFragment parses content as the inner HTML of a <body>, so no <html>/<head>/<body> wrappers are
// injected into the output (unlike html.Parse).
func parseFragment(content string) ([]*html.Node, error) {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	return html.ParseFragment(strings.NewReader(content), body)
}

// collectDistinctHrefs returns the unique, non-empty href values of every <a>, in first-seen order.
func collectDistinctHrefs(nodes []*html.Node) []string {
	seen := make(map[string]struct{})
	var hrefs []string
	for _, n := range nodes {
		walkAnchors(n, func(a *html.Node) {
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

// rewriteAnchors replaces the href of every <a> for which replace returns ok, using the value it
// returns.
func rewriteAnchors(root *html.Node, replace func(href string) (string, bool)) {
	walkAnchors(root, func(a *html.Node) {
		for i := range a.Attr {
			if a.Attr[i].Key != "href" {
				continue
			}
			if next, ok := replace(a.Attr[i].Val); ok {
				a.Attr[i].Val = next
			}
		}
	})
}

// walkAnchors invokes fn for every <a> element in the subtree rooted at n.
func walkAnchors(n *html.Node, fn func(anchor *html.Node)) {
	if n.Type == html.ElementNode && n.DataAtom == atom.A {
		fn(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkAnchors(c, fn)
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
