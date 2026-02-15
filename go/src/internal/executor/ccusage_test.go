package executor

import (
	"testing"
)

func TestEncodeCCProjectName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard path",
			input:    "/home/user/workspaces/trees/flux--task-abc123",
			expected: "-home-user-workspaces-trees-flux--task-abc123",
		},
		{
			name:     "path with dots",
			input:    "/Users/john.doe/projects/my.project/task.123",
			expected: "-Users-john-doe-projects-my-project-task-123",
		},
		{
			name:     "path with multiple dots",
			input:    "/var/www/site.example.com/deploy",
			expected: "-var-www-site-example-com-deploy",
		},
		{
			name:     "root path",
			input:    "/",
			expected: "-",
		},
		{
			name:     "simple path",
			input:    "/tmp",
			expected: "-tmp",
		},
		{
			name:     "path with trailing slash",
			input:    "/home/user/project/",
			expected: "-home-user-project-",
		},
		{
			name:     "complex mix",
			input:    "/opt/app.v2.0/task--abc-123/workspace.tmp",
			expected: "-opt-app-v2-0-task--abc-123-workspace-tmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeCCProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("EncodeCCProjectName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEncodeCCProjectNameEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only slashes",
			input:    "///",
			expected: "---",
		},
		{
			name:     "only dots",
			input:    "...",
			expected: "---",
		},
		{
			name:     "mixed separators",
			input:    "/./..//./",
			expected: "---------",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeCCProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("EncodeCCProjectName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
