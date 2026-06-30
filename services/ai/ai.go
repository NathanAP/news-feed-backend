// Package ai is the provider-agnostic seam for every AI call in the application. The rest of the
// codebase depends only on the Client interface — never on a concrete provider. Swapping Gemini
// for LangChain/Claude later means adding another implementation of Client and rewiring it in
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
	// ErrDisabled is returned by the disabled client when no provider is configured.
	ErrDisabled = errors.New("ai client is disabled: PRIMARY_AI_MODEL/GOOGLE_API_KEY not configured")
)

// Client is the seam for all AI interactions. Methods speak the domain (treat, keywords), never
// the provider's vocabulary.
type Client interface {
	// Treat returns the cleaned-up Markdown body of an article — form only, never substance.
	Treat(ctx context.Context, title, content string) (string, error)
	// Keywords returns 5-20 distinct keywords describing the (already treated) article.
	Keywords(ctx context.Context, title, content string) ([]string, error)
}

// NewDisabledClient returns a Client that fails every call with ErrDisabled. It lets the app boot
// and serve non-AI features when no provider is configured (e.g. local dev without an API key),
// instead of crashing at startup.
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
