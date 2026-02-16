package executor

import (
	"regexp"
	"strings"
)

// ExtractTaskSummary extracts the structured "## Task Summary" section from Claude CLI output.
//
// Claude CLI is prompted (via autopilot.txt) to generate a structured task summary with the format:
//
//	## Task Summary
//	### What Was Accomplished
//	### Key Changes
//	### Technical Decisions
//	### Verification
//	### Notes
//
// This function locates that section in the CLI output and extracts it for use in PR descriptions.
// The JSON result field from Claude CLI doesn't contain this structured summary, so we must
// parse it from the raw stdout text.

// taskSummaryRe matches the "## Task Summary" section heading.
var taskSummaryRe = regexp.MustCompile(`(?m)^##\s+Task Summary\s*$`)

// nextSectionRe matches the next ## heading (to mark end of task summary section).
var nextSectionRe = regexp.MustCompile(`(?m)^##\s+\w`)

func ExtractTaskSummary(output string) string {
	// Find the "## Task Summary" heading
	summaryLoc := taskSummaryRe.FindStringIndex(output)
	if summaryLoc == nil {
		return ""
	}

	// Start after the "## Task Summary" line
	startIdx := summaryLoc[1]
	if startIdx >= len(output) {
		return ""
	}

	// Find the next section (next ## heading)
	remainingOutput := output[startIdx:]
	nextSectionLoc := nextSectionRe.FindStringIndex(remainingOutput)

	var summaryText string
	if nextSectionLoc != nil {
		// Extract up to next section
		summaryText = remainingOutput[:nextSectionLoc[0]]
	} else {
		// Extract to end of output
		summaryText = remainingOutput
	}

	// Clean up the extracted text
	summaryText = strings.TrimSpace(summaryText)

	// Remove any trailing JSON output (Claude CLI appends JSON at the end)
	// Look for the start of JSON: a line that starts with '{'
	lines := strings.Split(summaryText, "\n")
	cleanedLines := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Stop if we hit JSON output (starts with '{' and looks like JSON)
		if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"type"`) {
			break
		}
		cleanedLines = append(cleanedLines, line)
	}
	summaryText = strings.Join(cleanedLines, "\n")
	summaryText = strings.TrimSpace(summaryText)

	return summaryText
}
