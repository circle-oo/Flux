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

// SystemPromptData holds data for system.txt template.
type SystemPromptData struct {
	ProjectName        string
	ProjectDescription string
	ProjectTechStack   string
	GoalID             string
	GoalTitle          string
	GoalDescription    string
	TaskType           string
	Priority           int
}

// AutopilotData holds data for autopilot.txt template.
type AutopilotData struct {
	Title              string
	Description        string
	TriageAnalysis     string
	Prompt             string
	ProjectName        string
	ProjectDescription string
	ProjectTechStack   string
	TaskType           string
}
