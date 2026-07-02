package external

import (
	"context"

	"github.com/nathanap/news-feed-backend/services/ai"
)

// MockAIClient implements ai.Client for tests. By default Treat echoes the content with a marker,
// Keywords returns five fixed keywords and Judge returns a fixed passing score; override the funcs
// to simulate failures or specific outputs.
type MockAIClient struct {
	TreatFn     func(ctx context.Context, title, content string) (string, error)
	KeywordsFn  func(ctx context.Context, title, content string) ([]string, error)
	JudgeFn     func(ctx context.Context, feedKeywords []string, title string, articleKeywords []string, content string) (int, error)
	TranslateFn func(ctx context.Context, targetLanguage, personality, title, content string) (ai.Translation, error)
}

func (m *MockAIClient) Treat(ctx context.Context, title, content string) (string, error) {
	if m.TreatFn != nil {
		return m.TreatFn(ctx, title, content)
	}
	return "treated: " + content, nil
}

func (m *MockAIClient) Keywords(ctx context.Context, title, content string) ([]string, error) {
	if m.KeywordsFn != nil {
		return m.KeywordsFn(ctx, title, content)
	}
	return []string{"alpha", "beta", "gamma", "delta", "epsilon"}, nil
}

func (m *MockAIClient) Judge(ctx context.Context, feedKeywords []string, title string, articleKeywords []string, content string) (int, error) {
	if m.JudgeFn != nil {
		return m.JudgeFn(ctx, feedKeywords, title, articleKeywords, content)
	}
	return 90, nil
}

func (m *MockAIClient) Translate(ctx context.Context, targetLanguage, personality, title, content string) (ai.Translation, error) {
	if m.TranslateFn != nil {
		return m.TranslateFn(ctx, targetLanguage, personality, title, content)
	}
	return ai.Translation{
		Title:   "translated: " + title,
		Content: "translated: " + content,
	}, nil
}

var _ ai.Client = (*MockAIClient)(nil)
