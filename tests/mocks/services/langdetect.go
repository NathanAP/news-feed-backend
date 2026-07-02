package services

import "github.com/nathanap/news-feed-backend/services/langdetect"

// MockLanguageDetector implements langdetect.Detector for tests. By default it reports Portuguese;
// set DetectFn to simulate other languages or a failed detection.
type MockLanguageDetector struct {
	DetectFn func(title, content string) (string, bool)
}

func (m *MockLanguageDetector) Detect(title, content string) (string, bool) {
	if m.DetectFn != nil {
		return m.DetectFn(title, content)
	}
	return "pt", true
}

var _ langdetect.Detector = (*MockLanguageDetector)(nil)
