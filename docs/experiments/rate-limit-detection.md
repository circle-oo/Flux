# Rate Limit Detection Experiment (Task 2A.13)

## Objective

Document Claude Code rate limit behavior: exit codes, stderr patterns, and timing. Provide findings for Phase 2B rate limit response implementation.

## Methodology

Due to cost and disruption concerns, this experiment uses observation-based analysis rather than intentionally triggering rate limits. Data gathered from:
1. Claude Code CLI documentation and error handling
2. Smoke test JSON response structure analysis
3. Known Anthropic API rate limit patterns

## Observed Response Structure

From `--output-format json`, errors return:
```json
{
  "type": "result",
  "subtype": "error_max_turns" | "error_tool" | ...,
  "is_error": true,
  "result": "error message",
  "session_id": "...",
  "total_cost_usd": 0.0
}
```

## Detection Patterns (Spec + Observed)

### Exit Code Detection
| Exit Code | Meaning | Confidence |
|-----------|---------|------------|
| 0 | Success | Known |
| 1 | General error | Known |
| 429 | Rate limit (hypothesized) | Unverified — Anthropic API returns 429 HTTP, unclear if CLI maps this to exit code |

### Stderr Pattern Detection
| Pattern | Description | Confidence |
|---------|-------------|------------|
| `rate limit` | Rate limit exceeded | High — standard Anthropic error message |
| `too many requests` | HTTP 429 equivalent | High |
| `429` | Numeric code in error | Medium |
| `capacity` | Server capacity exceeded | Medium |
| `try again` | Retry suggestion | Low — could match non-rate-limit errors |
| `overloaded` | Model overloaded | Medium — Anthropic uses this for capacity issues |

### JSON Response Detection (New — based on v2.1.42 output)
| Field | Check | Confidence |
|-------|-------|------------|
| `is_error: true` | Error occurred | Known |
| `subtype` contains "rate" | Rate limit specific error | Unverified |
| `result` contains rate limit patterns | Error message text | High |

## Current Implementation

In `internal/executor/claude_code.go`:
```go
func IsRateLimited(exitCode int, stderr string) bool {
    if exitCode == 429 { return true }
    patterns := []string{"rate limit", "too many requests", "429", "capacity", "try again"}
    lower := strings.ToLower(stderr)
    for _, p := range patterns { if strings.Contains(lower, p) { return true } }
    return false
}
```

## Recommendations for Phase 2B

1. **Add JSON response parsing for rate limits**: Check `is_error` and `result` fields from the JSON output, not just stderr
2. **Log all error responses**: During Phase 2A operation, log full JSON + stderr for any non-zero exit code to build a real-world pattern library
3. **Conservative approach**: Current multi-pattern detection is good; false positives (treating non-rate-limit errors as rate limits) are safer than false negatives
4. **Monitor `--fallback-model` flag**: v2.1.42 supports `--fallback-model` which could auto-switch to a less loaded model instead of failing
5. **Consider `--max-budget-usd`**: Budget limits could prevent excessive spending during rate limit recovery loops

## Timing Observations

- Smoke test execution: ~6.3 seconds for a trivial prompt
- Model used: opus (62K context tokens for system prompt alone)
- Cost per trivial call: ~$0.39 (due to large system prompt caching)
- Expected rate limit window: 5 hours (Anthropic max_5x plan)

## Open Questions (for Phase 2B)

1. What exact exit code does Claude Code return on rate limit?
2. Does the `--fallback-model` flag handle rate limits automatically?
3. Can we detect rate limits from the JSON `subtype` field?
4. What is the stderr output format for rate limit errors?

These will be answered through production observation during Phase 2A operation.
