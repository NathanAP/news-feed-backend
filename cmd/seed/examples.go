package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// examples mirrors cmd/seed/examples.json: the sample data used by the seed commands. Each command
// reads only the section it needs. Articles carry no url_original (generated randomly per run) nor
// source_id (linked to the sources that exist in the DB at seed time).
type examples struct {
	User            exampleUser            `json:"user"`
	UserPreferences exampleUserPreferences `json:"user_preferences"`
	Sources         []exampleSource        `json:"sources"`
	Feeds           []exampleFeed          `json:"feeds"`
	Articles        []exampleArticle       `json:"articles"`
}

type exampleUser struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type exampleUserPreferences struct {
	Theme            string `json:"theme"`
	Language         string `json:"language"`
	TranslateContent bool   `json:"translate_content"`
	AIPersonality    string `json:"ai_personality"`
}

type exampleSource struct {
	URL    string `json:"url"`
	URLRss string `json:"url_rss"`
}

type exampleFeed struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

type exampleArticle struct {
	Title            string   `json:"title"`
	Content          string   `json:"content"`
	Keywords         []string `json:"keywords"`
	LanguageOriginal string   `json:"language_original"`
}

// loadExamples reads and parses cmd/seed/examples.json located next to the seed sources.
func loadExamples(projectRoot string) (examples, error) {
	path := filepath.Join(projectRoot, "cmd", "seed", "examples.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return examples{}, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var ex examples
	if err := json.Unmarshal(data, &ex); err != nil {
		return examples{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return ex, nil
}
