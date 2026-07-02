// Package ai is the provider-agnostic seam for every AI call in the application. The rest of the
// codebase depends only on these interfaces — never on a concrete provider. Each capability is its
// own interface so different tasks can use different providers (e.g. treatment on an LLM, keywords
// on a local SLM). Swapping/adding a provider means adding an implementation and rewiring in
// main.go; nothing else changes.
package ai

import (
	"context"
	"errors"
)

var (
	// ErrEmptyTreatment is returned when the model produces no usable treated content.
	ErrEmptyTreatment = errors.New("treatment returned empty content")
	// ErrInvalidKeywords is returned when the model's keyword output cannot be parsed or fails
	// the 5-20 distinct-items rule.
	ErrInvalidKeywords = errors.New("keywords output is invalid")
	// ErrInvalidScore is returned when the model's judgement output cannot be parsed into an
	// integer score in the 0-100 range.
	ErrInvalidScore = errors.New("judgement score output is invalid")
	// ErrInvalidTranslation is returned when the model's translation output cannot be parsed or is
	// missing the title/content.
	ErrInvalidTranslation = errors.New("translation output is invalid")
	// ErrDisabled is returned by the disabled client when no provider is configured.
	ErrDisabled = errors.New("ai client is disabled: provider/model not configured")
)

// Translation is the result of translating an article: its title and content (HTML preserved)
// rendered in the target language, for display only (never persisted). Keywords are NOT translated —
// they are stored canonically in English and translating them for display would only produce terms
// out of sync with the stored ones.
type Translation struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Treater cleans up an article's content (form only, never substance).
type Treater interface {
	Treat(ctx context.Context, title, content string) (string, error)
}

// Keyworder assigns 5-20 distinct keywords to an (already treated) article.
type Keyworder interface {
	Keywords(ctx context.Context, title, content string) ([]string, error)
}

// Judger scores how strongly an article belongs to a feed (0-100), given the feed's keywords and
// the article's title, keywords and content. It is the second judgement layer (after the cheap
// keyword-overlap filter), so the caller passes only feeds that already survived stage 1.
type Judger interface {
	Judge(ctx context.Context, feedKeywords []string, title string, articleKeywords []string, content string) (int, error)
}

// Translator translates an article's title and content into the target language (an English
// language name, e.g. "Spanish"), adapting the tone to the given personality. It must preserve the
// content's HTML structure and never summarize or persist. Used on demand, LLM-only.
type Translator interface {
	Translate(ctx context.Context, targetLanguage, personality, title, content string) (Translation, error)
}

// Client is a provider that can do all capabilities — convenient for providers (Gemini, Ollama)
// that implement every one. Wiring picks each capability independently.
type Client interface {
	Treater
	Keyworder
	Judger
	Translator
}

// NewDisabledClient returns a Client that fails every call with ErrDisabled. It lets the app boot
// and serve non-AI features when a provider is not configured, instead of crashing at startup.
func NewDisabledClient() Client {
	return disabledClient{}
}

type disabledClient struct{}

func (disabledClient) Treat(context.Context, string, string) (string, error) {
	return "", ErrDisabled
}

func (disabledClient) Keywords(context.Context, string, string) ([]string, error) {
	return nil, ErrDisabled
}

func (disabledClient) Judge(context.Context, []string, string, []string, string) (int, error) {
	return 0, ErrDisabled
}

func (disabledClient) Translate(context.Context, string, string, string, string) (Translation, error) {
	return Translation{}, ErrDisabled
}
