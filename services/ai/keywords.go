package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nathanap/news-feed-backend/schemas"
)

// ParseKeywords extracts the keyword list from a model's raw output and enforces the 5-20 distinct
// rule. It is shared across providers so every AI implementation validates keywords the same way.
// It tolerates: a bare JSON array (`["a","b"]`), an object with a `keywords` field
// (`{"keywords":["a","b"]}`), and a ```json ... ``` code fence around either.
func ParseKeywords(raw string) ([]string, error) {
	cleaned := stripCodeFence(strings.TrimSpace(raw))

	decoded, err := decodeKeywordList(cleaned)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKeywords, err)
	}

	seen := make(map[string]struct{}, len(decoded))
	keywords := make([]string, 0, len(decoded))
	for _, k := range decoded {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		key := strings.ToLower(k)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keywords = append(keywords, k)
	}

	if len(keywords) < schemas.ArticleKeywordsMin || len(keywords) > schemas.ArticleKeywordsMax {
		return nil, fmt.Errorf("%w: got %d, want %d-%d", ErrInvalidKeywords, len(keywords), schemas.ArticleKeywordsMin, schemas.ArticleKeywordsMax)
	}
	return keywords, nil
}

func decodeKeywordList(s string) ([]string, error) {
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err == nil {
		return arr, nil
	}

	var obj struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return nil, err
	}
	return obj.Keywords, nil
}

func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimPrefix(s, "json")
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
