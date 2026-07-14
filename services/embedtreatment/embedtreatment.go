// Package embedtreatment turns script-based embeds — the ones that can only render by loading
// third-party JavaScript — into a plain link to the original post, so no <script> is ever needed in
// the stored content. Today it handles Instagram (`<blockquote class="instagram-media">` + a loader
// <script>): the blockquote becomes `<a href="{permalink}">{permalink}</a>` and the loader script is
// dropped downstream by sanitize. It runs before sanitization, is deterministic (no AI, no DB) and
// best-effort: any parse/render error returns the original content unchanged.
//
// iframe embeds (YouTube/Twitch) are NOT handled here — those are safe to keep as-is and are
// allowlisted directly by services/sanitize.
package embedtreatment

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/nathanap/news-feed-backend/logger"
)

// Treat converts known script-based embeds in content into plain links. Returns the original content
// (plus the error, for logging) if the HTML cannot be parsed or rendered.
func Treat(content string, verbose bool) (string, error) {
	nodes, err := parseFragment(content)
	if err != nil {
		return content, err
	}

	var blockquotes []*html.Node
	for _, n := range nodes {
		collectInstagram(n, &blockquotes)
	}
	if len(blockquotes) == 0 {
		return content, nil
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

	out, err := renderFragment(nodes)
	if err != nil {
		return content, err
	}
	if verbose {
		logger.Print(fmt.Sprintf("@@@ EMBED TREATMENT - %d script embed(s) converted to link @@@", converted), logger.ColorCyan)
	}
	return out, nil
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
	forEachAnchor(bq, func(a *html.Node) {
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

func forEachAnchor(n *html.Node, fn func(*html.Node)) {
	if n.Type == html.ElementNode && n.DataAtom == atom.A {
		fn(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		forEachAnchor(c, fn)
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
