package external

import (
	"context"

	"github.com/nathanap/news-feed-backend/services/ai"
)

// MockAIClient implements ai.Client for tests. By default Treat echoes the content with a marker
// and Keywords returns five fixed keywords; override the funcs to simulate failures or specific
// outputs.
type MockAIClient struct {
	TreatFn    func(ctx context.Context, title, content string) (string, error)
	KeywordsFn func(ctx context.Context, title, content string) ([]string, error)
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

var _ ai.Client = (*MockAIClient)(nil)
