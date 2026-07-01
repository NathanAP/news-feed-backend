// Package gemini is the Google Gemini implementation of ai.Client. It is the ONLY package that
// imports the genai SDK — all Gemini-specific details are confined here so the rest of the app
// stays provider-agnostic.
package gemini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sethvargo/go-retry"
	"google.golang.org/genai"

	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/prompts"
)

const (
	treatmentPromptName = "article_treatment"
	keywordsPromptName  = "article_keywords"

	requestTimeout = 60 * time.Second
)

// Client implements ai.Client using the Gemini API.
type Client struct {
	genai *genai.Client
	model string
}

var _ ai.Client = (*Client)(nil)

// NewClient builds a Gemini-backed ai.Client for the given model (e.g. "gemini-2.5-flash").
func NewClient(ctx context.Context, apiKey, model string) (*Client, error) {
	gc, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}
	return &Client{genai: gc, model: model}, nil
}

// Treat runs the treatment prompt and returns the cleaned Markdown body.
func (c *Client) Treat(ctx context.Context, title, content string) (string, error) {
	out, err := c.generate(ctx, treatmentPromptName, map[string]string{"title": title, "content": content})
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", ai.ErrEmptyTreatment
	}
	return out, nil
}

// Keywords runs the keyword prompt against the (already treated) content and validates the output.
func (c *Client) Keywords(ctx context.Context, title, content string) ([]string, error) {
	out, err := c.generate(ctx, keywordsPromptName, map[string]string{"title": title, "content": content})
	if err != nil {
		return nil, err
	}
	return ai.ParseKeywords(out)
}

func (c *Client) generate(ctx context.Context, promptName string, vars map[string]string) (string, error) {
	prompt, err := prompts.Load(promptName)
	if err != nil {
		return "", err
	}

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(prompts.Render(prompt.System, vars), genai.RoleUser),
	}
	user := genai.Text(prompts.Render(prompt.User, vars))

	var text string
	backoff := retry.WithMaxRetries(2, retry.NewExponential(500*time.Millisecond))
	err = retry.Do(ctx, backoff, func(ctx context.Context) error {
		callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, genErr := c.genai.Models.GenerateContent(callCtx, c.model, user, cfg)
		if genErr != nil {
			return retry.RetryableError(genErr)
		}
		text = resp.Text()
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("gemini generate (%s) failed: %w", promptName, err)
	}
	return text, nil
}
