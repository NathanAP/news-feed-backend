// Package openaicompat implements ai.Client against any OpenAI-compatible chat endpoint. It is
// used for the local SLM (Ollama) today, but the same client points at any OpenAI-compatible
// server (Groq, vLLM, Together, ...) just by changing the base URL — so it is the single seam for
// self-hosted / third-party small models.
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/sethvargo/go-retry"

	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/prompts"
)

const (
	treatmentPromptName = "article_treatment"
	keywordsPromptName  = "article_keywords"

	requestTimeout = 120 * time.Second // local SLMs on CPU can be slow
)

// thinkingPattern matches the <think>...</think> block that reasoning models like Qwen3 may emit
// before the answer; it would break JSON parsing, so we strip it.
var thinkingPattern = regexp.MustCompile(`(?s)<think>.*?</think>`)

// Client talks to an OpenAI-compatible /v1/chat/completions endpoint.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

var _ ai.Client = (*Client)(nil)

// NewClient builds a client for the given base URL (e.g. http://localhost:11434 for Ollama) and
// model (e.g. qwen3:4b).
func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: requestTimeout},
	}
}

func (c *Client) Treat(ctx context.Context, title, content string) (string, error) {
	out, err := c.chat(ctx, treatmentPromptName, map[string]string{"title": title, "content": content})
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(stripThinking(out))
	if out == "" {
		return "", ai.ErrEmptyTreatment
	}
	return out, nil
}

func (c *Client) Keywords(ctx context.Context, title, content string) ([]string, error) {
	out, err := c.chat(ctx, keywordsPromptName, map[string]string{"title": title, "content": content})
	if err != nil {
		return nil, err
	}
	return ai.ParseKeywords(stripThinking(out))
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *Client) chat(ctx context.Context, promptName string, vars map[string]string) (string, error) {
	prompt, err := prompts.Load(promptName)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: prompts.Render(prompt.System, vars)},
			{Role: "user", Content: prompts.Render(prompt.User, vars)},
		},
		Stream: false,
	})
	if err != nil {
		return "", err
	}

	var content string
	backoff := retry.WithMaxRetries(2, retry.NewExponential(500*time.Millisecond))
	err = retry.Do(ctx, backoff, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return err // not retryable
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return retry.RetryableError(err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return retry.RetryableError(err)
		}
		if resp.StatusCode != http.StatusOK {
			return retry.RetryableError(fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
		}

		var parsed chatResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return retry.RetryableError(err)
		}
		if len(parsed.Choices) == 0 {
			return retry.RetryableError(fmt.Errorf("empty choices in response"))
		}
		content = parsed.Choices[0].Message.Content
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("openai-compatible generate (%s) failed: %w", promptName, err)
	}
	return content, nil
}

func stripThinking(s string) string {
	return thinkingPattern.ReplaceAllString(s, "")
}
