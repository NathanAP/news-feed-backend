// Package prompts loads and renders the YAML prompt files used by the AI layer. Keeping the
// prompts as embedded data (not Go strings) makes them provider-agnostic: any AI implementation
// (Gemini today, LangChain/Claude tomorrow) consumes the same files.
package prompts

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var files embed.FS

// Prompt mirrors the structure of a prompt YAML file.
type Prompt struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	System      string `yaml:"system"`
	User        string `yaml:"user"`
}

// Load reads and parses a prompt by its base name (without the .yaml extension).
func Load(name string) (Prompt, error) {
	data, err := files.ReadFile(name + ".yaml")
	if err != nil {
		return Prompt{}, fmt.Errorf("prompt %q not found: %w", name, err)
	}

	var p Prompt
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Prompt{}, fmt.Errorf("failed to parse prompt %q: %w", name, err)
	}
	return p, nil
}

// Render substitutes {{key}} placeholders in the template with the given values.
func Render(template string, vars map[string]string) string {
	out := template
	for key, value := range vars {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return out
}
