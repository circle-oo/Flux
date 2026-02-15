// Package prompts provides embedded prompt templates for the executor.
// Edit the .txt files directly to improve prompts without touching Go code.
package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.txt
var promptFS embed.FS

var templates *template.Template

func init() {
	var err error
	templates, err = template.ParseFS(promptFS, "*.txt")
	if err != nil {
		panic(fmt.Sprintf("failed to parse prompt templates: %v", err))
	}
}

// Render executes a named template with the given data and returns the result.
func Render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", name, err)
	}
	return buf.String(), nil
}

// MustRender is like Render but panics on error. Use for templates that should never fail.
func MustRender(name string, data any) string {
	s, err := Render(name, data)
	if err != nil {
		panic(err)
	}
	return s
}

// SystemPromptData holds data for system.txt template.
type SystemPromptData struct {
	GoalID   string
	TaskType string
	Priority int
}

// TriageData holds data for triage.txt template.
type TriageData struct {
	Title       string
	Type        string
	Priority    int
	Description string
	Tags        string
}

// AutopilotData holds data for autopilot.txt template.
type AutopilotData struct {
	Title          string
	Description    string
	TriageAnalysis string
	Prompt         string
}
