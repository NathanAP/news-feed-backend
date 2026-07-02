package ai

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ParseScore extracts the 0-100 relevance score from a model's raw judgement output. It is shared
// across providers so every AI implementation validates the score the same way. It tolerates: a
// JSON object with a `score` field (`{"score":80}`), a bare integer (`80`), and a ```json ... ```
// code fence around either. Scores outside 0-100 are rejected.
func ParseScore(raw string) (int, error) {
	cleaned := stripCodeFence(strings.TrimSpace(raw))

	score, err := decodeScore(cleaned)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidScore, err)
	}

	if score < 0 || score > 100 {
		return 0, fmt.Errorf("%w: got %d, want 0-100", ErrInvalidScore, score)
	}
	return score, nil
}

func decodeScore(s string) (int, error) {
	var obj struct {
		Score *int `json:"score"`
	}
	if err := json.Unmarshal([]byte(s), &obj); err == nil && obj.Score != nil {
		return *obj.Score, nil
	}

	// Fall back to a bare integer (some models ignore the JSON-object instruction).
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n, nil
	}
	return 0, fmt.Errorf("no integer score found in %q", s)
}
