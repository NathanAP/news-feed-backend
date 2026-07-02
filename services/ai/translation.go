package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseTranslation extracts the translated title and content from a model's raw output. It is
// shared across providers so every AI implementation validates the shape the same way. It tolerates
// a ```json ... ``` code fence and requires a non-empty title and content.
func ParseTranslation(raw string) (Translation, error) {
	cleaned := stripCodeFence(strings.TrimSpace(raw))

	var out struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return Translation{}, fmt.Errorf("%w: %v", ErrInvalidTranslation, err)
	}

	out.Title = strings.TrimSpace(out.Title)
	out.Content = strings.TrimSpace(out.Content)
	if out.Title == "" || out.Content == "" {
		return Translation{}, fmt.Errorf("%w: title and content are required", ErrInvalidTranslation)
	}

	return Translation{Title: out.Title, Content: out.Content}, nil
}
