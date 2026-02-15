package server

import (
	"strings"
	"testing"
)

func TestValidateTaskInput(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid input",
			title:       "Implement user authentication",
			description: "Add JWT-based authentication with refresh tokens",
			wantErr:     false,
		},
		{
			name:        "empty title",
			title:       "",
			description: "Some description",
			wantErr:     true,
			errContains: "title is required",
		},
		{
			name:        "oversized title",
			title:       strings.Repeat("a", 501),
			description: "Description",
			wantErr:     true,
			errContains: "title exceeds maximum length",
		},
		{
			name:        "oversized description",
			title:       "Title",
			description: strings.Repeat("a", 10241),
			wantErr:     true,
			errContains: "description exceeds maximum length",
		},
		{
			name:        "SQL injection in title",
			title:       "Delete user'; DROP TABLE users--",
			description: "Description",
			wantErr:     true,
			errContains: "malicious SQL pattern",
		},
		{
			name:        "SQL injection in description",
			title:       "Valid title",
			description: "This query: SELECT * FROM users WHERE id=1 OR 1=1",
			wantErr:     true,
			errContains: "malicious SQL pattern",
		},
		{
			name:        "SQL injection UNION SELECT",
			title:       "Test",
			description: "UNION SELECT password FROM users",
			wantErr:     true,
			errContains: "malicious SQL pattern",
		},
		{
			name:        "shell injection subshell",
			title:       "Run $(( cat /etc/passwd ))",
			description: "Description",
			wantErr:     true,
			errContains: "malicious shell pattern",
		},
		{
			name:        "shell injection pipe to bash",
			title:       "Test | bash",
			description: "Description",
			wantErr:     true,
			errContains: "malicious shell pattern",
		},
		{
			name:        "shell injection rm command",
			title:       "Title",
			description: "Run this: ; rm -rf /tmp",
			wantErr:     true,
			errContains: "malicious shell pattern",
		},
		{
			name:        "legitimate bash code snippet",
			title:       "Fix deployment script",
			description: "Update the script to use proper variable expansion: echo $HOME and cd $WORKDIR",
			wantErr:     false,
		},
		{
			name:        "legitimate code with backticks in markdown",
			title:       "Document API usage",
			description: "Use the following code: ```bash\necho hello\n```",
			wantErr:     false,
		},
		{
			name:        "whitespace-only title",
			title:       "   ",
			description: "Description",
			wantErr:     true,
			errContains: "title is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaskInput(tt.title, tt.description)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateTaskInput() expected error containing %q, got nil", tt.errContains)
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateTaskInput() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateTaskInput() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidatePrompt(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid prompt",
			prompt:  "Analyze this code for security issues",
			wantErr: false,
		},
		{
			name:        "empty prompt",
			prompt:      "",
			wantErr:     true,
			errContains: "prompt is required",
		},
		{
			name:        "whitespace-only prompt",
			prompt:      "   \n  \t  ",
			wantErr:     true,
			errContains: "prompt is required",
		},
		{
			name:        "oversized prompt",
			prompt:      strings.Repeat("a", 10241),
			wantErr:     true,
			errContains: "prompt exceeds maximum length",
		},
		{
			name:    "max size prompt",
			prompt:  strings.Repeat("a", 10240),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePrompt(tt.prompt)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePrompt() expected error containing %q, got nil", tt.errContains)
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidatePrompt() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePrompt() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "trims whitespace",
			input: "  hello world  ",
			want:  "hello world",
		},
		{
			name:  "removes null bytes",
			input: "hello\x00world",
			want:  "helloworld",
		},
		{
			name:  "combined trimming and null removal",
			input: "  test\x00data\x00  ",
			want:  "testdata",
		},
		{
			name:  "already clean",
			input: "clean input",
			want:  "clean input",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeInput(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeInput() = %q, want %q", got, tt.want)
			}
		})
	}
}
