// Package sanitize enforces the stored-article HTML whitelist with bluemonday. Since the treatment
// pipeline no longer runs the article body through an LLM (the AI never touches the body), this is
// the single, deterministic guarantee that persisted content is safe: it keeps rich-but-safe markup
// (basic formatting, links, images and a tight allowlist of video embeds) and strips anything
// dangerous or unknown. Hard invariants, enforced by the policy below: never <script>, never event
// handlers (on*), never <style>, and <iframe> only when its src matches the known-embed allowlist
// (any other iframe is dropped).
package sanitize

import (
	"html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// textPolicy strips every tag, leaving only text — used to feed plain text to the AI (keywords),
// which does not need the markup and would otherwise pay tokens for image/iframe/href URLs.
var textPolicy = bluemonday.StrictPolicy()

// embedSrc is the allowlist of trusted <iframe> embed providers (YouTube, Twitch). Only iframes whose
// src matches are kept; every other iframe has its src stripped and is dropped. It never permits
// <script>, so no third-party JS ever runs — script-based embeds (Instagram) are turned into a plain
// link upstream (services/embedtreatment).
var embedSrc = regexp.MustCompile(`^https://(www\.youtube\.com/embed/|www\.youtube-nocookie\.com/embed/|player\.twitch\.tv/|clips\.twitch\.tv/embed)`)

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

	// Known video embeds: keep <iframe> ONLY when its src matches the trusted allowlist (YouTube,
	// Twitch). An iframe with any other src has its src stripped and is dropped (unwrapped). Display
	// attributes only — never on*/style, and never <script>.
	p.AllowAttrs("src").Matching(embedSrc).OnElements("iframe")
	p.AllowAttrs("width", "height", "allowfullscreen", "allow", "title", "loading").OnElements("iframe")

	// Drop these elements together with their contents (otherwise raw CSS/JS/head text would leak
	// through as plain text).
	p.SkipElementsContent("script", "style", "head", "title", "noscript")
	return p
}

// Sanitize returns the input HTML with only the whitelisted safe markup kept: basic formatting,
// links (safe schemes, rel=nofollow), images and allowlisted video embeds (YouTube/Twitch iframes).
// Scripts, styles, non-allowlisted iframes, event handlers and unknown attributes are stripped; a
// full HTML document is handled gracefully (wrappers removed, body content preserved).
func Sanitize(htmlContent string) string {
	return policy.Sanitize(htmlContent)
}

// PlainText strips all HTML and returns the readable text, collapsed to single-spaced words. It is
// what the keyword AI is fed: the model does not need markup, and dropping tags/URLs (image srcs,
// iframe srcs, hrefs) cuts a large share of the input tokens with no loss of semantic signal.
func PlainText(htmlContent string) string {
	text := html.UnescapeString(textPolicy.Sanitize(htmlContent))
	return strings.Join(strings.Fields(text), " ")
}
