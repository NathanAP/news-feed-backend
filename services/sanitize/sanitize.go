// Package sanitize enforces the treated-article HTML whitelist with bluemonday. It is the reliable
// (and XSS-safe) guarantee that the stored content contains only basic formatting tags — the LLM
// prompt asks for that, but a model can never be trusted to enforce it. NOTE: bluemonday's
// UGCPolicy is intentionally NOT used here: it allows <a>, <img> and URLs, which this project's
// rules forbid ("sem URLs / sem a / sem img"). We build a stricter custom policy instead.
package sanitize

import (
	"context"

	"github.com/microcosm-cc/bluemonday"

	"github.com/nathanap/news-feed-backend/services/ai"
)

// policy is safe for concurrent use once built, so it is created once at package init.
var policy = buildPolicy()

func buildPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	// Basic formatting elements only, with no attributes (so no class/style). No <a>, <img>, and no
	// document wrappers (<html>/<body>/<head>) — those are stripped, their text kept.
	p.AllowElements(
		"p", "br",
		"strong", "b", "em", "i", "u", "s",
		"ul", "ol", "li",
		"blockquote", "h2", "h3", "h4",
		"code", "pre",
	)
	// Drop these elements together with their contents (otherwise raw CSS/JS/head text would leak
	// through as plain text when a full HTML document arrives from the model).
	p.SkipElementsContent("script", "style", "head", "title", "noscript", "iframe")
	return p
}

// Sanitize returns the input HTML with only the whitelisted basic tags kept. A full HTML document
// (with <html>/<body>/<head>) is handled gracefully: the wrappers are removed and the body content
// is preserved; scripts, styles, links, images and attributes are stripped.
func Sanitize(html string) string {
	return policy.Sanitize(html)
}

// NewTreater wraps a Treater so its output is always sanitized. Applying it at the seam keeps both
// the CRON pipeline and the dry-run endpoint consistent.
func NewTreater(inner ai.Treater) ai.Treater {
	return sanitizingTreater{inner: inner}
}

type sanitizingTreater struct {
	inner ai.Treater
}

func (s sanitizingTreater) Treat(ctx context.Context, title, content string) (string, error) {
	out, err := s.inner.Treat(ctx, title, content)
	if err != nil {
		return "", err
	}
	return Sanitize(out), nil
}
