// Package langdetect detects the original language of an article's text. It is a thin wrapper over
// lingua-go, restricted to the languages the application supports (CLAUDE.md), and returns lowercase
// ISO 639-1 codes that line up with the schemas.Language enum. Detection is deterministic, offline
// and quota-free — it is NOT an AI call, so it stays provider-agnostic and cheap.
package langdetect

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
	lingua "github.com/pemistahl/lingua-go"
)

// Detector detects the language of an article's title+content.
type Detector interface {
	// Detect returns the detected ISO 639-1 code (lowercase, e.g. "pt") and whether detection was
	// reliable. When a language cannot be reliably identified it returns ("", false) — the caller
	// then stores a null language_original.
	Detect(title, content string) (string, bool)
}

// tagStripper removes all HTML tags, leaving only text. Article content is HTML, and the tag names
// (<p>, <strong>, ...) would bias detection toward English, so we strip them before detecting.
var tagStripper = bluemonday.StrictPolicy()

type linguaDetector struct {
	inner lingua.LanguageDetector
}

// New builds a Detector restricted to the supported languages, with the language models preloaded so
// the first real detection does not pay the one-time loading cost.
func New() Detector {
	inner := lingua.NewLanguageDetectorBuilder().
		FromLanguages(
			lingua.Portuguese,
			lingua.English,
			lingua.Spanish,
			lingua.French,
			lingua.German,
			lingua.Italian,
		).
		WithPreloadedLanguageModels().
		Build()
	return &linguaDetector{inner: inner}
}

func (d *linguaDetector) Detect(title, content string) (string, bool) {
	text := strings.TrimSpace(title + "\n" + tagStripper.Sanitize(content))
	if text == "" {
		return "", false
	}

	lang, ok := d.inner.DetectLanguageOf(text)
	if !ok {
		return "", false
	}
	return strings.ToLower(lang.IsoCode639_1().String()), true
}
