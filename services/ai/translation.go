package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseTranslation extracts the translated title, content and keywords from a model's raw output.
// It is shared across providers so every AI implementation validates the shape the same way. It
// tolerates a ```json ... ``` code fence and requires a non-empty title and content. Keywords are
// optional (an article could, in theory, arrive without any) and are trimmed.
func ParseTranslation(raw string) (Translation, error) {
	cleaned := stripCodeFence(strings.TrimSpace(raw))

	var out struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return Translation{}, fmt.Errorf("%w: %v", ErrInvalidTranslation, err)
	}

	out.Title = strings.TrimSpace(out.Title)
	out.Content = strings.TrimSpace(out.Content)
	if out.Title == "" || out.Content == "" {
		return Translation{}, fmt.Errorf("%w: title and content are required", ErrInvalidTranslation)
	}

	keywords := make([]string, 0, len(out.Keywords))
	for _, k := range out.Keywords {
		if k = strings.TrimSpace(k); k != "" {
			keywords = append(keywords, k)
		}
	}

	return Translation{Title: out.Title, Content: out.Content, Keywords: keywords}, nil
}
