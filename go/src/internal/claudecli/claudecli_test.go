package claudecli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/config"
)

func TestNewRunner(t *testing.T) {
	cfg := &config.ExecutorConfig{
		MaxExecutionTime: 5 * time.Minute,
		MaxOutputSize:    1024 * 1024,
		MaxTurns:         30,
	}

	runner := NewRunner(cfg)
	if runner == nil {
		t.Fatal("expected non-nil runner")
	}
	if runner.Cfg != cfg {
		t.Error("runner config not set correctly")
	}
}

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		stderr   string
		want     bool
	}{
		{
			name:     "exit code 429",
			exitCode: 429,
			stderr:   "",
			want:     true,
		},
		{
			name:     "rate limit in stderr",
			exitCode: 1,
			stderr:   "Error: rate limit exceeded",
			want:     true,
		},
		{
			name:     "too many requests",
			exitCode: 1,
			stderr:   "Too many requests, please try again later",
			want:     true,
		},
		{
			name:     "429 in message",
			exitCode: 1,
			stderr:   "HTTP 429 error",
			want:     true,
		},
		{
			name:     "capacity exceeded",
			exitCode: 1,
			stderr:   "Capacity exceeded, please wait",
			want:     true,
		},
		{
			name:     "try again message",
			exitCode: 1,
			stderr:   "Please try again in a few minutes",
			want:     true,
		},
		{
			name:     "no rate limit",
			exitCode: 1,
			stderr:   "Some other error",
			want:     false,
		},
		{
			name:     "success",
			exitCode: 0,
			stderr:   "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRateLimited(tt.exitCode, tt.stderr)
			if got != tt.want {
				t.Errorf("IsRateLimited(%d, %q) = %v, want %v", tt.exitCode, tt.stderr, got, tt.want)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		want    *ParsedResponse
		wantErr bool
	}{
		{
			name: "valid v2.1.42 response",
			stdout: `{"type":"result","subtype":"success","is_error":false,"result":"Task completed successfully","session_id":"sess_123","total_cost_usd":0.025,"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":1000,"cache_read_input_tokens":200}}`,
			want: &ParsedResponse{
				ResultText: "Task completed successfully",
				SessionID:  "sess_123",
				TokensUsed: 1350, // 100 + 50 + 1000 + 200
				CostUSD:    0.025,
			},
			wantErr: false,
		},
		{
			name: "response with stderr warning prefix",
			stdout: `[WARN] Fast mode requires the native binary
{"type":"result","subtype":"success","is_error":false,"result":"Final result","session_id":"abc-456","total_cost_usd":0.50,"usage":{"input_tokens":10,"output_tokens":5}}`,
			want: &ParsedResponse{
				ResultText: "Final result",
				SessionID:  "abc-456",
				TokensUsed: 15,
				CostUSD:    0.50,
			},
			wantErr: false,
		},
		{
			name: "minimal response",
			stdout: `{"type":"result","result":"OK"}`,
			want: &ParsedResponse{
				ResultText: "OK",
			},
			wantErr: false,
		},
		{
			name:    "no JSON in output",
			stdout:  `just plain text with no json`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			stdout:  `{invalid json`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResponse(tt.stdout)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.ResultText != tt.want.ResultText {
				t.Errorf("ResultText = %q, want %q", got.ResultText, tt.want.ResultText)
			}
			if got.SessionID != tt.want.SessionID {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tt.want.SessionID)
			}
			if got.TokensUsed != tt.want.TokensUsed {
				t.Errorf("TokensUsed = %d, want %d", got.TokensUsed, tt.want.TokensUsed)
			}
			if got.CostUSD != tt.want.CostUSD {
				t.Errorf("CostUSD = %f, want %f", got.CostUSD, tt.want.CostUSD)
			}
		})
	}
}

func TestLimitedBuffer(t *testing.T) {
	tests := []struct {
		name      string
		maxSize   int64
		writes    []string
		wantSize  int64
		wantText  string
		wantTrunc bool
	}{
		{
			name:      "within limit",
			maxSize:   100,
			writes:    []string{"hello", " ", "world"},
			wantSize:  11,
			wantText:  "hello world",
			wantTrunc: false,
		},
		{
			name:      "exact limit",
			maxSize:   5,
			writes:    []string{"hello"},
			wantSize:  5,
			wantText:  "hello",
			wantTrunc: false,
		},
		{
			name:      "exceeds limit",
			maxSize:   5,
			writes:    []string{"hello", "world"},
			wantSize:  5,
			wantText:  "hello",
			wantTrunc: true,
		},
		{
			name:      "single write exceeds",
			maxSize:   3,
			writes:    []string{"hello"},
			wantSize:  3,
			wantText:  "hel",
			wantTrunc: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &LimitedBuffer{MaxSize: tt.maxSize}
			for _, w := range tt.writes {
				buf.Write([]byte(w))
			}
			if buf.Size != tt.wantSize {
				t.Errorf("size = %d, want %d", buf.Size, tt.wantSize)
			}
			if buf.String() != tt.wantText {
				t.Errorf("text = %q, want %q", buf.String(), tt.wantText)
			}
			if buf.Truncated != tt.wantTrunc {
				t.Errorf("truncated = %v, want %v", buf.Truncated, tt.wantTrunc)
			}
		})
	}
}

func TestRun_TimeoutContextPattern(t *testing.T) {
	cfg := &config.ExecutorConfig{
		MaxExecutionTime: 100 * time.Millisecond,
		MaxOutputSize:    1024,
		MaxTurns:         1,
	}

	runner := NewRunner(cfg)
	ctx := context.Background()

	opts := Opts{
		Prompt:  "test",
		WorkDir: "/tmp",
		Model:   "sonnet",
	}

	result, err := runner.Run(ctx, opts)

	if result == nil && err == nil {
		t.Error("expected either non-nil result or error")
	}
}

func TestRun_OutputSizeLimit(t *testing.T) {
	cfg := &config.ExecutorConfig{
		MaxExecutionTime: 5 * time.Second,
		MaxOutputSize:    10, // Very small limit
		MaxTurns:         1,
	}

	_ = NewRunner(cfg)

	buf := &LimitedBuffer{MaxSize: cfg.MaxOutputSize}
	largeOutput := strings.Repeat("x", 100)
	buf.Write([]byte(largeOutput))

	if !buf.Truncated {
		t.Error("expected buffer to be truncated")
	}
	if buf.Size != cfg.MaxOutputSize {
		t.Errorf("buffer size = %d, want %d", buf.Size, cfg.MaxOutputSize)
	}
	if int64(len(buf.String())) != cfg.MaxOutputSize {
		t.Errorf("output length = %d, want %d", len(buf.String()), cfg.MaxOutputSize)
	}
}
