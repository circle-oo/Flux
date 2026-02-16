package executor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/circle-oo/flux/internal/apiclient"
)

// Decomposition represents a task that should be broken into subtasks.
type Decomposition struct {
	Decompose bool              `json:"decompose"`
	Subtasks  []DecomposedTask  `json:"subtasks"`
}

// DecomposedTask represents a single subtask in a decomposition plan.
type DecomposedTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// ParseDecomposition attempts to extract a decomposition plan from Claude's response.
// Returns nil if no valid decomposition is found (indicating normal code output).
func ParseDecomposition(resultText string) *Decomposition {
	slog.Debug("parsing decomposition from result", "result_length", len(resultText))
	// Look for the decompose JSON pattern
	if !strings.Contains(resultText, `"decompose"`) {
		slog.Debug("no decomposition keyword found in result")
		return nil
	}

	jsonText := extractDecompositionJSON(resultText)
	if jsonText == "" {
		slog.Warn("decomposition keyword found but could not extract valid JSON from result")
		return nil
	}

	// Unmarshal the JSON
	var decomp Decomposition
	if err := json.Unmarshal([]byte(jsonText), &decomp); err != nil {
		slog.Warn("failed to unmarshal decomposition JSON", "error", err, "json_length", len(jsonText))
		return nil
	}

	// Validate decomposition
	if !decomp.Decompose {
		return nil
	}

	if len(decomp.Subtasks) == 0 {
		return nil
	}

	// Enforce max 5 subtasks
	if len(decomp.Subtasks) > 5 {
		slog.Warn("decomposition exceeded max subtasks, truncating", "original_count", len(decomp.Subtasks))
		decomp.Subtasks = decomp.Subtasks[:5]
	}

	// Enforce min 2 subtasks (no point in decomposing into 1)
	if len(decomp.Subtasks) < 2 {
		slog.Warn("decomposition has too few subtasks, rejecting", "count", len(decomp.Subtasks))
		return nil
	}

	// Trim whitespace from titles and descriptions
	for i := range decomp.Subtasks {
		decomp.Subtasks[i].Title = strings.TrimSpace(decomp.Subtasks[i].Title)
		decomp.Subtasks[i].Description = strings.TrimSpace(decomp.Subtasks[i].Description)
	}

	// Validate subtask quality
	if err := ValidateSubtasks(decomp.Subtasks); err != nil {
		slog.Warn("subtask validation failed, rejecting decomposition", "error", err)
		return nil
	}

	slog.Info("decomposition detected", "subtask_count", len(decomp.Subtasks))
	return &decomp
}

// extractDecompositionJSON extracts the decomposition JSON from Claude's response text.
// It handles two formats:
//  1. JSON embedded in a markdown code block (```json\n{...}\n```) — Claude's typical output
//  2. Inline JSON starting with {"decompose" or { "decompose"
//
// Returns empty string if no valid JSON block is found.
func extractDecompositionJSON(resultText string) string {
	// Strategy 1: Extract from markdown code blocks (most common case).
	// Claude typically formats the decomposition inside ```json ... ``` blocks.
	if jsonText := extractJSONFromCodeBlock(resultText); jsonText != "" {
		return jsonText
	}

	// Strategy 2: Look for inline JSON starting with {"decompose" or { "decompose"
	jsonStart := strings.Index(resultText, `{"decompose"`)
	if jsonStart == -1 {
		jsonStart = strings.Index(resultText, `{ "decompose"`)
	}
	if jsonStart == -1 {
		return ""
	}

	jsonEnd := findMatchingBrace(resultText, jsonStart)
	if jsonEnd == 0 {
		return ""
	}

	return resultText[jsonStart:jsonEnd]
}

// extractJSONFromCodeBlock extracts JSON containing "decompose" from a markdown code block.
// Looks for ```json\n...\n``` or ```\n...\n``` patterns.
func extractJSONFromCodeBlock(text string) string {
	// Try ```json first, then plain ```
	for _, fence := range []string{"```json", "```"} {
		startIdx := 0
		for {
			fencePos := strings.Index(text[startIdx:], fence)
			if fencePos == -1 {
				break
			}
			fencePos += startIdx

			// Find end of fence opening line
			contentStart := strings.Index(text[fencePos:], "\n")
			if contentStart == -1 {
				break
			}
			contentStart += fencePos + 1

			// Find closing ```
			closingFence := strings.Index(text[contentStart:], "```")
			if closingFence == -1 {
				break
			}

			content := strings.TrimSpace(text[contentStart : contentStart+closingFence])

			// Check if this code block contains a decomposition
			if strings.Contains(content, `"decompose"`) && strings.HasPrefix(content, "{") {
				// Validate it's parseable JSON before returning
				var test map[string]interface{}
				if json.Unmarshal([]byte(content), &test) == nil {
					return content
				}
				slog.Debug("code block contains decompose keyword but invalid JSON", "content_length", len(content))
			}

			// Move past this code block and keep searching
			startIdx = contentStart + closingFence + 3
		}
	}
	return ""
}

// findMatchingBrace finds the matching closing brace for a JSON object starting at jsonStart.
// Returns the index after the closing brace, or 0 if not found.
func findMatchingBrace(text string, jsonStart int) int {
	braceCount := 0
	inString := false
	escaped := false

	for i := jsonStart; i < len(text); i++ {
		ch := text[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if ch == '{' {
			braceCount++
		} else if ch == '}' {
			braceCount--
			if braceCount == 0 {
				return i + 1
			}
		}
	}
	return 0
}

// ValidateSubtasks checks that all subtasks meet quality criteria.
func ValidateSubtasks(subtasks []DecomposedTask) error {
	vagueTitles := []string{"other", "misc", "additional", "extra", "remaining", "fix issues", "cleanup"}

	for i, subtask := range subtasks {
		// Check for empty or vague titles
		if subtask.Title == "" {
			return fmt.Errorf("subtask %d has empty title", i+1)
		}

		titleLower := strings.ToLower(subtask.Title)
		for _, vague := range vagueTitles {
			if strings.Contains(titleLower, vague) && len(strings.Fields(subtask.Title)) <= 3 {
				return fmt.Errorf("subtask %d has vague title: %q", i+1, subtask.Title)
			}
		}

		// Check for empty descriptions
		if subtask.Description == "" {
			return fmt.Errorf("subtask %d has empty description", i+1)
		}

		// Check for too-short descriptions (likely not actionable)
		if len(subtask.Description) < 20 {
			return fmt.Errorf("subtask %d description too short (< 20 chars): %q", i+1, subtask.Description)
		}

		// Check for meta-tasks that shouldn't be separate subtasks
		metaTasks := []string{"write tests", "add tests", "documentation", "deploy", "commit", "push"}
		for _, meta := range metaTasks {
			if strings.Contains(titleLower, meta) {
				return fmt.Errorf("subtask %d appears to be a meta-task: %q (tests/docs should be part of implementation)", i+1, subtask.Title)
			}
		}
	}

	// Check for duplicate titles
	titleSet := make(map[string]bool)
	for i, subtask := range subtasks {
		normalized := strings.ToLower(strings.TrimSpace(subtask.Title))
		if titleSet[normalized] {
			return fmt.Errorf("subtask %d has duplicate title: %q", i+1, subtask.Title)
		}
		titleSet[normalized] = true
	}

	return nil
}

// ToSubtaskRequests converts a Decomposition to a slice of apiclient.SubtaskRequest.
func ToSubtaskRequests(d *Decomposition) []apiclient.SubtaskRequest {
	if d == nil {
		return nil
	}

	requests := make([]apiclient.SubtaskRequest, len(d.Subtasks))
	for i, task := range d.Subtasks {
		requests[i] = apiclient.SubtaskRequest{
			Title:       task.Title,
			Description: task.Description,
		}
	}
	return requests
}
