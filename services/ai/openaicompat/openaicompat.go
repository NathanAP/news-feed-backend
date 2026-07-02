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
	treatmentPromptName   = "article_treatment"
	keywordsPromptName    = "article_keywords"
	judgementPromptName   = "article_feed_judgement"
	translationPromptName = "article_translation"

	requestTimeout = 120 * time.Second // local SLMs on CPU can be slow
)

// thinkingPattern matches the <think>...</think> block that reasoning models like Qwen3 may emit
// before the answer; it would break JSON parsing, so we strip it.
var thinkingPattern = regexp.MustCompile(`(?s)<think>.*?</think>`)

// Client talks to an OpenAI-compatible /v1/chat/completions endpoint.
type Client struct {
	baseURL string
	model   string
	apiKey  string
	http    *http.Client
}

var _ ai.Client = (*Client)(nil)

// NewClient builds a client for the given base URL (e.g. http://localhost:11434 for Ollama, or a
// hosted provider like Groq) and model. apiKey is optional: local Ollama needs none; hosted
// providers require it (sent as a Bearer token).
func NewClient(baseURL, model, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: requestTimeout},
	}
}

func (c *Client) Treat(ctx context.Context, title, content string) (string, error) {
	out, err := c.chat(ctx, treatmentPromptName, map[string]string{"title": title, "content": content}, false)
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
	// jsonMode forces a JSON object, which both prevents the invalid-JSON failures and (via
	// grammar-constrained decoding) stops the model from emitting a reasoning block.
	out, err := c.chat(ctx, keywordsPromptName, map[string]string{"title": title, "content": content}, true)
	if err != nil {
		return nil, err
	}
	return ai.ParseKeywords(stripThinking(out))
}

func (c *Client) Judge(ctx context.Context, feedKeywords []string, title string, articleKeywords []string, content string) (int, error) {
	// jsonMode forces the {"score": N} object, avoiding prose and the reasoning block.
	out, err := c.chat(ctx, judgementPromptName, map[string]string{
		"feed_keywords":    strings.Join(feedKeywords, ", "),
		"title":            title,
		"article_keywords": strings.Join(articleKeywords, ", "),
		"content":          content,
	}, true)
	if err != nil {
		return 0, err
	}
	return ai.ParseScore(stripThinking(out))
}

func (c *Client) Translate(ctx context.Context, targetLanguage, personality, title, content string, keywords []string) (ai.Translation, error) {
	// jsonMode forces the {title, content, keywords} object.
	out, err := c.chat(ctx, translationPromptName, map[string]string{
		"target_language": targetLanguage,
		"personality":     personality,
		"title":           title,
		"keywords":        strings.Join(keywords, ", "),
		"content":         content,
	}, true)
	if err != nil {
		return ai.Translation{}, err
	}
	return ai.ParseTranslation(stripThinking(out))
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Stream         bool            `json:"stream"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *Client) chat(ctx context.Context, promptName string, vars map[string]string, jsonMode bool) (string, error) {
	prompt, err := prompts.Load(promptName)
	if err != nil {
		return "", err
	}

	// "/no_think" is Qwen3's soft switch to skip the reasoning block — a big speedup on CPU for
	// non-reasoning tasks. It is harmless for models that do not support it.
	system := prompts.Render(prompt.System, vars) + "\n\n/no_think"

	req := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompts.Render(prompt.User, vars)},
		},
		Stream: false,
	}
	if jsonMode {
		req.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	endpoint := c.chatCompletionsURL()

	var content string
	backoff := retry.WithMaxRetries(2, retry.NewExponential(500*time.Millisecond))
	err = retry.Do(ctx, backoff, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return err // not retryable
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

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

// chatCompletionsURL builds the endpoint, tolerating base URLs that already include the "/v1"
// segment (e.g. Groq's https://api.groq.com/openai/v1) as well as those that do not (e.g. Ollama's
// http://localhost:11434) — avoiding a duplicated "/v1/v1".
func (c *Client) chatCompletionsURL() string {
	if strings.HasSuffix(c.baseURL, "/v1") {
		return c.baseURL + "/chat/completions"
	}
	return c.baseURL + "/v1/chat/completions"
}

func stripThinking(s string) string {
	return thinkingPattern.ReplaceAllString(s, "")
}
