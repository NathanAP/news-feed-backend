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
	// ErrDisabled is returned by the disabled client when no provider is configured.
	ErrDisabled = errors.New("ai client is disabled: provider/model not configured")
)

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

// Client is a provider that can do all capabilities — convenient for providers (Gemini, Ollama)
// that implement every one. Wiring picks a Treater, a Keyworder and a Judger independently.
type Client interface {
	Treater
	Keyworder
	Judger
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
