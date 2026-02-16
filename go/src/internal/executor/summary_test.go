package executor

import (
	"testing"
)

func TestExtractTaskSummary(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name: "valid task summary",
			output: `Some initial output

## Task Summary

### What Was Accomplished
Fixed the authentication bug by updating the JWT validation logic.

### Key Changes
- File: auth/jwt.go - Updated token validation
- File: auth/middleware.go - Added error handling

### Technical Decisions
Used bcrypt for password hashing instead of SHA-256.

### Verification
- Build: PASSED
- Tests: PASSED

{"type":"result","result":"success"}`,
			want: `### What Was Accomplished
Fixed the authentication bug by updating the JWT validation logic.

### Key Changes
- File: auth/jwt.go - Updated token validation
- File: auth/middleware.go - Added error handling

### Technical Decisions
Used bcrypt for password hashing instead of SHA-256.

### Verification
- Build: PASSED
- Tests: PASSED`,
		},
		{
			name: "no task summary section",
			output: `Just some output
no summary here
{"type":"result","result":"completed"}`,
			want: "",
		},
		{
			name: "task summary at end of output",
			output: `Some work done

## Task Summary

### What Was Accomplished
Implemented the new feature.

### Key Changes
- File: feature.go - Added implementation`,
			want: `### What Was Accomplished
Implemented the new feature.

### Key Changes
- File: feature.go - Added implementation`,
		},
		{
			name: "task summary followed by another section",
			output: `## Task Summary

### What Was Accomplished
Added new tests.

## Other Section

Some other content`,
			want: `### What Was Accomplished
Added new tests.`,
		},
		{
			name: "empty task summary",
			output: `## Task Summary

## Next Section`,
			want: "",
		},
		{
			name: "task summary with JSON at end",
			output: `## Task Summary

### What Was Accomplished
Completed the task successfully.

{"type":"result","subtype":"success","result":"done"}`,
			want: `### What Was Accomplished
Completed the task successfully.`,
		},
		{
			name: "multiple ## headings",
			output: `## Initial Section

Some content

## Task Summary

### What Was Accomplished
Fixed issue #123

## Final Section

More content`,
			want: `### What Was Accomplished
Fixed issue #123`,
		},
		{
			name: "real world example with warnings",
			output: `[WARN] Fast mode enabled

I'll help fix the authentication bug.

## Task Summary

### What Was Accomplished
Fixed the JWT token validation to properly handle expired tokens. The authentication middleware now correctly rejects invalid tokens and returns appropriate error messages.

### Key Changes
- File: internal/auth/jwt.go:45 - Updated ValidateToken function to check expiration
- File: internal/auth/middleware.go:78 - Added error handling for token validation failures
- File: internal/auth/jwt_test.go:120 - Added test for expired token scenario

### Technical Decisions
Chose to use the standard jwt-go library's built-in expiration checking rather than implementing custom logic, as it's well-tested and follows JWT standards.

### Verification
- Build: PASSED
- Tests: PASSED (all 45 tests passing, including 3 new tests for token expiration)

### Notes
The fix ensures that expired tokens are properly rejected at the authentication layer before reaching the application logic.

{"type":"result","subtype":"success","is_error":false,"result":"Task completed successfully. Fixed JWT token validation to handle expired tokens properly.","session_id":"sess_abc123","total_cost_usd":0.025}`,
			want: `### What Was Accomplished
Fixed the JWT token validation to properly handle expired tokens. The authentication middleware now correctly rejects invalid tokens and returns appropriate error messages.

### Key Changes
- File: internal/auth/jwt.go:45 - Updated ValidateToken function to check expiration
- File: internal/auth/middleware.go:78 - Added error handling for token validation failures
- File: internal/auth/jwt_test.go:120 - Added test for expired token scenario

### Technical Decisions
Chose to use the standard jwt-go library's built-in expiration checking rather than implementing custom logic, as it's well-tested and follows JWT standards.

### Verification
- Build: PASSED
- Tests: PASSED (all 45 tests passing, including 3 new tests for token expiration)

### Notes
The fix ensures that expired tokens are properly rejected at the authentication layer before reaching the application logic.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTaskSummary(tt.output)
			if got != tt.want {
				t.Errorf("ExtractTaskSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTaskSummary_Integration(t *testing.T) {
	t.Run("real Claude CLI output format", func(t *testing.T) {
		// This is what Claude CLI actually outputs when using --output-format json
		output := `I'll fix the authentication bug by updating the JWT validation logic.

## Task Summary

### What Was Accomplished
Fixed the JWT token validation to properly handle expired tokens and invalid signatures. Updated the authentication middleware to return appropriate error codes for different failure scenarios.

### Key Changes
- File: internal/auth/jwt.go:45 - Added expiration time validation
- File: internal/auth/jwt.go:67 - Improved error messages for validation failures
- File: internal/auth/middleware.go:23 - Updated middleware to handle new error types
- File: internal/auth/jwt_test.go:89 - Added comprehensive test coverage

### Technical Decisions
Used the standard jwt-go library's expiration checking instead of implementing custom logic to ensure compatibility with JWT standards and reduce maintenance burden.

### Verification
- Build: PASSED
- Tests: PASSED (12/12 tests passing)

### Notes
All existing authentication flows remain backward compatible. The fix addresses the security vulnerability reported in issue #123.

{"type":"result","subtype":"success","is_error":false,"result":"Task completed successfully. Fixed JWT authentication vulnerability by implementing proper token expiration and signature validation.","session_id":"sess_xyz789","total_cost_usd":0.032,"usage":{"input_tokens":1250,"output_tokens":890,"cache_creation_input_tokens":5000,"cache_read_input_tokens":2500}}`

		want := `### What Was Accomplished
Fixed the JWT token validation to properly handle expired tokens and invalid signatures. Updated the authentication middleware to return appropriate error codes for different failure scenarios.

### Key Changes
- File: internal/auth/jwt.go:45 - Added expiration time validation
- File: internal/auth/jwt.go:67 - Improved error messages for validation failures
- File: internal/auth/middleware.go:23 - Updated middleware to handle new error types
- File: internal/auth/jwt_test.go:89 - Added comprehensive test coverage

### Technical Decisions
Used the standard jwt-go library's expiration checking instead of implementing custom logic to ensure compatibility with JWT standards and reduce maintenance burden.

### Verification
- Build: PASSED
- Tests: PASSED (12/12 tests passing)

### Notes
All existing authentication flows remain backward compatible. The fix addresses the security vulnerability reported in issue #123.`

		got := ExtractTaskSummary(output)
		if got != want {
			t.Errorf("ExtractTaskSummary() mismatch\nGot:\n%s\n\nWant:\n%s", got, want)
		}
	})
}

func TestExtractTaskSummary_EdgeCases(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		got := ExtractTaskSummary("")
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("only task summary heading", func(t *testing.T) {
		got := ExtractTaskSummary("## Task Summary\n")
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("task summary with extra whitespace", func(t *testing.T) {
		output := `## Task Summary

### What Was Accomplished
Fixed the bug.

`
		want := `### What Was Accomplished
Fixed the bug.`
		got := ExtractTaskSummary(output)
		if got != want {
			t.Errorf("ExtractTaskSummary() = %q, want %q", got, want)
		}
	})

	t.Run("case sensitive heading", func(t *testing.T) {
		// Should NOT match "task summary" (lowercase)
		output := `## task summary

Content here`
		got := ExtractTaskSummary(output)
		if got != "" {
			t.Errorf("expected empty string for lowercase heading, got %q", got)
		}
	})
}
