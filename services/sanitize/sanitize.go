// Package sanitize enforces the stored-article HTML whitelist with bluemonday. Since the treatment
// pipeline no longer runs the article body through an LLM (the AI never touches the body), this is
// the single, deterministic guarantee that persisted content is safe: it keeps rich-but-safe markup
// (basic formatting, links and images) and strips anything dangerous or unknown. Hard invariants,
// enforced by the policy below: never <script>, never event handlers (on*), never <style>, and never
// raw <iframe> (embeds are a separate, allowlisted concern handled in a later version).
package sanitize

import (
	"github.com/microcosm-cc/bluemonday"
)

// policy is safe for concurrent use once built, so it is created once at package init.
var policy = buildPolicy()

func buildPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	// Rich-but-safe formatting elements (no attributes, so no class/style/on*). Unlisted elements
	// (e.g. div, section) are unwrapped: their text is kept, the tag dropped.
	p.AllowElements(
		"p", "br", "hr", "span",
		"strong", "b", "em", "i", "u", "s", "sub", "sup", "small", "mark", "abbr", "cite", "q",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li", "dl", "dt", "dd",
		"blockquote", "code", "pre", "kbd", "samp",
		"figure", "figcaption",
		"table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col",
	)
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")

	// Links: keep <a href> for safe schemes only, tagged rel=nofollow. Internal article links (added
	// by the URL-treatment step in a later version) are plain http(s) URLs, so this covers them too.
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireNoFollowOnLinks(true)

	// Images: keep <img> with a safe src and descriptive attributes only. No srcset/class/style — the
	// stored markup stays minimal and the client controls presentation.
	p.AllowAttrs("src").OnElements("img")
	p.AllowAttrs("alt", "width", "height").OnElements("img")

	// Drop these elements together with their contents (otherwise raw CSS/JS/head text — and the
	// script-based embeds like Instagram — would leak through as plain text). <iframe> is dropped
	// here too until the allowlisted-embed version lands.
	p.SkipElementsContent("script", "style", "head", "title", "noscript", "iframe")
	return p
}

// Sanitize returns the input HTML with only the whitelisted safe markup kept: basic formatting,
// links (safe schemes, rel=nofollow) and images. Scripts, styles, iframes, event handlers and
// unknown attributes are stripped; a full HTML document is handled gracefully (wrappers removed,
// body content preserved).
func Sanitize(html string) string {
	return policy.Sanitize(html)
}
