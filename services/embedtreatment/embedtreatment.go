// Package embedtreatment makes known embeds safe and playable in our own client, before the content
// is sanitized. It does two deterministic things (no AI, no DB):
//
//  1. Script-based embeds (Instagram: `<blockquote class="instagram-media">` + a loader <script>)
//     become a plain link `<a href="{permalink}">{permalink}</a>` — no <script> is ever needed.
//  2. Twitch iframes (`player.twitch.tv` / `clips.twitch.tv`) get their `parent` query param rewritten
//     to our client's host. Twitch refuses to play unless `parent` matches the domain embedding the
//     iframe, and the RSS carries the source's domain (or none), so without this the player renders
//     but never plays. Needs the client URL; skipped when it is empty.
//
// YouTube iframes need no adjustment and are left as-is (sanitize allowlists them). It is best-effort:
// any parse/render error returns the original content unchanged.
package embedtreatment

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/nathanap/news-feed-backend/logger"
)

// Treat converts script embeds to links and fixes Twitch iframe `parent` params for the given client
// URL. Returns the original content (plus the error, for logging) if it cannot be parsed or rendered.
func Treat(content, clientURL string, verbose bool) (string, error) {
	nodes, err := parseFragment(content)
	if err != nil {
		return content, err
	}

	// 1. Twitch iframe parent -> our client's host (only if we know it).
	twitchFixed := 0
	if host := hostOf(clientURL); host != "" {
		for _, n := range nodes {
			twitchFixed += fixTwitchParents(n, host, verbose)
		}
	}

	// 2. Instagram blockquote -> link.
	var blockquotes []*html.Node
	for _, n := range nodes {
		collectInstagram(n, &blockquotes)
	}
	converted := 0
	for _, bq := range blockquotes {
		permalink := instagramPermalink(bq)
		if permalink == "" {
			continue // cannot extract the post URL: leave the node for sanitize to clean up
		}
		if replaceNode(bq, anchorNode(permalink), &nodes) {
			converted++
			if verbose {
				logger.Print("    embed -> link: "+permalink, logger.ColorGreen)
			}
		}
	}

	if twitchFixed == 0 && converted == 0 {
		return content, nil // nothing changed → return the original verbatim
	}

	out, err := renderFragment(nodes)
	if err != nil {
		return content, err
	}
	if verbose {
		logger.Print(fmt.Sprintf("@@@ EMBED TREATMENT - %d script embed(s) -> link, %d twitch parent(s) fixed @@@", converted, twitchFixed), logger.ColorCyan)
	}
	return out, nil
}

// fixTwitchParents rewrites the `parent` query param of every Twitch iframe in the subtree to host,
// returning how many were changed.
func fixTwitchParents(n *html.Node, host string, verbose bool) int {
	count := 0
	forEach(n, atom.Iframe, func(ifr *html.Node) {
		for i := range ifr.Attr {
			if ifr.Attr[i].Key != "src" {
				continue
			}
			if newSrc, ok := rewriteTwitchParent(ifr.Attr[i].Val, host); ok {
				ifr.Attr[i].Val = newSrc
				count++
				if verbose {
					logger.Print("    twitch parent -> "+host, logger.ColorGreen)
				}
			}
		}
	})
	return count
}

// rewriteTwitchParent forces `parent=host` (dropping any existing parent) on a Twitch player/clip URL.
// Returns (src, false) for non-Twitch URLs or when parent is already exactly host.
func rewriteTwitchParent(src, host string) (string, bool) {
	u, err := url.Parse(src)
	if err != nil {
		return src, false
	}
	if u.Host != "player.twitch.tv" && u.Host != "clips.twitch.tv" {
		return src, false
	}
	q := u.Query()
	if len(q["parent"]) == 1 && q.Get("parent") == host {
		return src, false // already correct
	}
	q.Set("parent", host) // replace every existing parent with a single, correct one
	u.RawQuery = q.Encode()
	return u.String(), true
}

// hostOf returns the hostname of a URL (no port), or "" when empty/unparseable.
func hostOf(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// collectInstagram appends every Instagram blockquote in the subtree to out.
func collectInstagram(n *html.Node, out *[]*html.Node) {
	if isInstagramBlockquote(n) {
		*out = append(*out, n)
		return // do not descend into an embed we are going to replace wholesale
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectInstagram(c, out)
	}
}

func isInstagramBlockquote(n *html.Node) bool {
	if n.Type != html.ElementNode || n.DataAtom != atom.Blockquote {
		return false
	}
	class, _ := getAttr(n, "class")
	return strings.Contains(class, "instagram-media")
}

// instagramPermalink returns the cleaned post URL for an Instagram blockquote: the
// data-instgrm-permalink attribute if present, otherwise the first inner <a> pointing at a post.
func instagramPermalink(bq *html.Node) string {
	if pl, ok := getAttr(bq, "data-instgrm-permalink"); ok && pl != "" {
		return cleanURL(pl)
	}
	var found string
	forEach(bq, atom.A, func(a *html.Node) {
		if found != "" {
			return
		}
		if href, ok := getAttr(a, "href"); ok && strings.Contains(href, "instagram.com/p/") {
			found = cleanURL(href)
		}
	})
	return found
}

// cleanURL drops the query string and fragment, keeping just the canonical post URL.
func cleanURL(raw string) string {
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		return raw[:i]
	}
	return raw
}

// anchorNode builds `<a href="url">url</a>`.
func anchorNode(url string) *html.Node {
	a := &html.Node{
		Type:     html.ElementNode,
		Data:     "a",
		DataAtom: atom.A,
		Attr:     []html.Attribute{{Key: "href", Val: url}},
	}
	a.AppendChild(&html.Node{Type: html.TextNode, Data: url})
	return a
}

// replaceNode swaps old for repl in the tree. Nested nodes are replaced via their parent; a
// top-level fragment node (no parent) is replaced in the nodes slice. Returns whether it replaced.
func replaceNode(old, repl *html.Node, nodes *[]*html.Node) bool {
	if old.Parent != nil {
		old.Parent.InsertBefore(repl, old)
		old.Parent.RemoveChild(old)
		return true
	}
	for i, n := range *nodes {
		if n == old {
			(*nodes)[i] = repl
			return true
		}
	}
	return false
}

func parseFragment(content string) ([]*html.Node, error) {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	return html.ParseFragment(strings.NewReader(content), body)
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

// forEach invokes fn for every element with atom a in the subtree rooted at n.
func forEach(n *html.Node, a atom.Atom, fn func(*html.Node)) {
	if n.Type == html.ElementNode && n.DataAtom == a {
		fn(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		forEach(c, a, fn)
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
