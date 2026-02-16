package prompts

import (
	"strings"
	"testing"
)

func TestRenderAutopilotPrompt(t *testing.T) {
	data := AutopilotData{
		Title:              "Test Task",
		Description:        "This is a test task description",
		TriageAnalysis:     "Test triage analysis",
		Prompt:             "Additional instructions",
		ProjectName:        "test-project",
		ProjectDescription: "Test project description",
		ProjectTechStack:   "Go, React",
	}

	result, err := Render("autopilot.txt", data)
	if err != nil {
		t.Fatalf("Failed to render autopilot prompt: %v", err)
	}

	// Verify key sections are present
	requiredSections := []string{
		"# Task: Test Task",
		"## Description",
		"## Triage Analysis",
		"## Additional Instructions",
		"## Execution Protocol",
		"### 1. ANALYZE",
		"### 2. DECIDE: Should this task be decomposed?",
		"### 3. PLAN",
		"### 4. IMPLEMENT",
		"### 5. VERIFY",
		"### 6. SUMMARIZE",
		"## Task Summary",
		"### What Was Accomplished",
		"### Key Changes",
		"### Technical Decisions",
		"### Verification",
		"**Summary Guidelines:**",
		"## Rules",
	}

	for _, section := range requiredSections {
		if !strings.Contains(result, section) {
			t.Errorf("Expected prompt to contain section %q, but it was not found", section)
		}
	}

	// Verify the new summary section includes important guidance
	if !strings.Contains(result, "Keep it concise but informative (aim for 150-300 words total)") {
		t.Error("Expected summary guidelines to include conciseness guidance")
	}

	if !strings.Contains(result, "Include actual file paths (use file:line format for specific references)") {
		t.Error("Expected summary guidelines to mention file:line format")
	}

	if !strings.Contains(result, "Build: [PASSED/FAILED/SKIPPED]") {
		t.Error("Expected summary template to include verification status format")
	}
}

func TestRenderAutopilotPromptWithoutOptionalFields(t *testing.T) {
	data := AutopilotData{
		Title:       "Minimal Task",
		Description: "Minimal description",
	}

	result, err := Render("autopilot.txt", data)
	if err != nil {
		t.Fatalf("Failed to render autopilot prompt with minimal data: %v", err)
	}

	// Should still contain essential sections
	if !strings.Contains(result, "# Task: Minimal Task") {
		t.Error("Expected prompt to contain task title")
	}

	if !strings.Contains(result, "### 6. SUMMARIZE") {
		t.Error("Expected prompt to contain summarize section")
	}

	// Optional sections should not appear if empty
	if strings.Contains(result, "## Triage Analysis\n\n") {
		t.Error("Empty triage analysis section should not be rendered")
	}

	if strings.Contains(result, "## Additional Instructions\n\n") {
		t.Error("Empty additional instructions section should not be rendered")
	}
}

func TestRenderSystemPrompt(t *testing.T) {
	data := SystemPromptData{
		ProjectName:        "test-project",
		ProjectDescription: "A test project",
		ProjectTechStack:   "Go, TypeScript",
		GoalID:             "goal-123",
		GoalTitle:          "Test Goal",
		GoalDescription:    "Test goal description",
		Priority:           50,
	}

	result, err := Render("system.txt", data)
	if err != nil {
		t.Fatalf("Failed to render system prompt: %v", err)
	}

	// Verify key components
	if !strings.Contains(result, "autonomous coding agent") {
		t.Error("Expected system prompt to mention autonomous coding agent")
	}

	if !strings.Contains(result, "test-project") {
		t.Error("Expected system prompt to include project name")
	}

	if !strings.Contains(result, "Task Priority: 50") {
		t.Error("Expected system prompt to include priority")
	}
}
