package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/circle-oo/flux/internal/config"
)

// ClaudeCodeRunner executes Claude Code CLI commands.
type ClaudeCodeRunner struct {
	cfg *config.ExecutorConfig
}

// ClaudeCodeResult contains the result of a Claude Code CLI execution.
type ClaudeCodeResult struct {
	ExitCode   int
	Stdout     string
	Stderr     string
	Duration   time.Duration
	TokensUsed int
	CostUSD    float64
	SessionID  string
}

// ClaudeCodeOpts contains options for running Claude Code CLI.
type ClaudeCodeOpts struct {
	Prompt       string
	WorkDir      string
	Model        string // "sonnet" or "opus"
	SystemPrompt string // Goal + context injected here
}

// ParsedResponse contains parsed fields from Claude Code JSON output.
type ParsedResponse struct {
	ResultText string
	SessionID  string
	TokensUsed int
	CostUSD    float64
}

// NewClaudeCodeRunner creates a new Claude Code CLI runner.
func NewClaudeCodeRunner(cfg *config.ExecutorConfig) *ClaudeCodeRunner {
	return &ClaudeCodeRunner{cfg: cfg}
}

// Run executes the Claude Code CLI with the given options.
func (r *ClaudeCodeRunner) Run(ctx context.Context, opts ClaudeCodeOpts) (*ClaudeCodeResult, error) {
	slog.Info("starting claude code CLI", "model", opts.Model, "workdir", opts.WorkDir, "has_system_prompt", opts.SystemPrompt != "")

	// Create timeout context BEFORE exec.CommandContext (CRITICAL TIMEOUT BUG FIX)
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.MaxExecutionTime)
	defer cancel()

	// Build command arguments
	// Note: --cwd and --max-turns don't exist in v2.1.42
	// Use cmd.Dir for working directory instead
	args := []string{
		"-p", opts.Prompt,
		"--model", opts.Model,
		"--output-format", "json",
		"--dangerously-skip-permissions",
	}
	slog.Debug("claude code CLI args", "args", args)

	if opts.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.SystemPrompt)
	}

	cmd := exec.CommandContext(timeoutCtx, "claude", args...)
	cmd.Dir = opts.WorkDir // Set working directory via cmd.Dir

	// Set process group to kill child processes on timeout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Create limited buffers for stdout/stderr to prevent memory exhaustion
	stdoutBuf := &limitedBuffer{maxSize: r.cfg.MaxOutputSize}
	stderrBuf := &limitedBuffer{maxSize: r.cfg.MaxOutputSize}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &ClaudeCodeResult{
		ExitCode: 0,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
	}

	// Extract exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("command execution failed: %w", err)
		}
	}

	slog.Info("claude code CLI completed", "exit_code", result.ExitCode, "duration", duration, "stdout_size", len(result.Stdout), "stderr_size", len(result.Stderr))

	// Check output size guardrail
	if stdoutBuf.truncated {
		slog.Warn("stdout exceeded max output size", "max_bytes", r.cfg.MaxOutputSize)
		return result, fmt.Errorf("stdout exceeded max output size (%d bytes)", r.cfg.MaxOutputSize)
	}

	// Parse JSON response if available
	if result.Stdout != "" && result.ExitCode == 0 {
		parsed, parseErr := ParseResponse(result.Stdout)
		if parseErr == nil {
			result.SessionID = parsed.SessionID
			result.TokensUsed = parsed.TokensUsed
			result.CostUSD = parsed.CostUSD
			slog.Debug("claude code parsed response", "session_id", result.SessionID, "tokens", result.TokensUsed, "cost_usd", result.CostUSD)
		} else {
			slog.Warn("failed to parse claude code response", "error", parseErr)
		}
		// Don't fail on parse errors - just log them (caller should handle)
	}

	return result, nil
}

// ParseResponse parses the Claude Code --output-format json response.
// v2.1.42 returns a single JSON object with fields:
//
//	{type, subtype, is_error, result, session_id, total_cost_usd, usage{input_tokens, output_tokens, ...}, ...}
func ParseResponse(stdout string) (*ParsedResponse, error) {
	// The response may have stderr warnings before the JSON (e.g., "[WARN] Fast mode...")
	// Find the first '{' to start JSON parsing
	jsonStart := strings.Index(stdout, "{")
	if jsonStart < 0 {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	slog.Debug("parsing claude code response", "json_start_offset", jsonStart)
	jsonStr := stdout[jsonStart:]

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	result := &ParsedResponse{}

	// Extract result text
	if r, ok := resp["result"].(string); ok {
		result.ResultText = r
	}

	// Extract session_id
	if sid, ok := resp["session_id"].(string); ok {
		result.SessionID = sid
	}

	// Extract cost
	if cost, ok := resp["total_cost_usd"].(float64); ok {
		result.CostUSD = cost
	}

	// Extract token counts from usage
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		var total int
		if inp, ok := usage["input_tokens"].(float64); ok {
			total += int(inp)
		}
		if cache, ok := usage["cache_creation_input_tokens"].(float64); ok {
			total += int(cache)
		}
		if cacheRead, ok := usage["cache_read_input_tokens"].(float64); ok {
			total += int(cacheRead)
		}
		if out, ok := usage["output_tokens"].(float64); ok {
			total += int(out)
		}
		result.TokensUsed = total
	}

	return result, nil
}

// IsRateLimited checks if the error is due to rate limiting.
func IsRateLimited(exitCode int, stderr string) bool {
	slog.Debug("checking rate limit", "exit_code", exitCode, "stderr_length", len(stderr))
	if exitCode == 429 {
		slog.Warn("rate limit detected", "exit_code", exitCode)
		return true
	}

	lower := strings.ToLower(stderr)
	rateLimitPatterns := []string{"rate limit", "too many requests", "429", "capacity", "try again"}
	for _, pattern := range rateLimitPatterns {
		if strings.Contains(lower, pattern) {
			slog.Warn("rate limit detected", "exit_code", exitCode)
			return true
		}
	}

	return false
}

// limitedBuffer is a buffer that enforces a maximum size.
type limitedBuffer struct {
	buf       strings.Builder
	maxSize   int64
	size      int64
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (n int, err error) {
	if b.truncated {
		return len(p), nil // Discard further writes
	}

	available := b.maxSize - b.size
	if int64(len(p)) > available {
		// Write only what fits
		b.buf.Write(p[:available])
		b.size = b.maxSize
		b.truncated = true
		return len(p), nil // Report full write to avoid errors
	}

	n, err = b.buf.Write(p)
	b.size += int64(n)
	return n, err
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}
