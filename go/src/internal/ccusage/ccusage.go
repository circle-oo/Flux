// Package ccusage provides a Go wrapper around the ccusage CLI tool
// for querying Claude Code token usage and cost data.
// ccusage reads local JSONL logs and calculates costs from model pricing,
// making it the single source of truth for usage data regardless of billing plan.
// See: https://ccusage.com
package ccusage

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// cmdTimeout is the maximum time a ccusage command is allowed to run.
// ccusage scans local JSONL logs and routinely takes 10-20s, so this must
// be generous. All callers are background (BillingCache, async executor,
// rate limit handler) — nothing on a hot path.
const cmdTimeout = 60 * time.Second

// BillingCache runs ccusage queries in the background and serves cached results.
// The billing API handler reads from this cache instead of shelling out per request.
// Data is persisted to the ccusage_cache DB table so it survives restarts.
type BillingCache struct {
	ccusageCmd string
	db         *sql.DB
	interval   time.Duration

	mu    sync.RWMutex
	daily *DailyEntry
	block *BlockEntry

	stop chan struct{}
	done chan struct{}
}

// NewBillingCache creates a cache that refreshes ccusage data every interval.
// If db is non-nil, cached data is persisted to the ccusage_cache table.
func NewBillingCache(ccusageCmd string, db *sql.DB, interval time.Duration) *BillingCache {
	return &BillingCache{
		ccusageCmd: ccusageCmd,
		db:         db,
		interval:   interval,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Start begins background refresh. It loads persisted data from DB first
// (instant data on startup), then spawns a background loop that refreshes
// from ccusage CLI on the configured interval.
func (c *BillingCache) Start() {
	go func() {
		defer close(c.done)
		c.loadFromDB()
		c.refresh()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.refresh()
			case <-c.stop:
				return
			}
		}
	}()
}

// Stop shuts down the background goroutine.
func (c *BillingCache) Stop() {
	close(c.stop)
	<-c.done
}

// Daily returns the cached daily entry (may be nil if not yet fetched or ccusage failed).
func (c *BillingCache) Daily() *DailyEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.daily
}

// Block returns the cached active block entry (may be nil).
func (c *BillingCache) Block() *BlockEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.block
}

// loadFromDB reads persisted cache rows from the ccusage_cache table.
func (c *BillingCache) loadFromDB() {
	if c.db == nil {
		return
	}

	rows, err := c.db.Query(`SELECT key, data FROM ccusage_cache`)
	if err != nil {
		slog.Debug("billing_cache: failed to load from DB", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, data string
		if err := rows.Scan(&key, &data); err != nil {
			continue
		}
		switch key {
		case "daily":
			var entry DailyEntry
			if json.Unmarshal([]byte(data), &entry) == nil {
				c.mu.Lock()
				c.daily = &entry
				c.mu.Unlock()
				slog.Debug("billing_cache: loaded daily from DB")
			}
		case "block":
			var entry BlockEntry
			if json.Unmarshal([]byte(data), &entry) == nil {
				c.mu.Lock()
				c.block = &entry
				c.mu.Unlock()
				slog.Debug("billing_cache: loaded block from DB")
			}
		}
	}
}

// persistToDB writes the current cache values to the ccusage_cache table.
func (c *BillingCache) persistToDB(daily *DailyEntry, block *BlockEntry) {
	if c.db == nil {
		return
	}

	if daily != nil {
		if data, err := json.Marshal(daily); err == nil {
			_, err := c.db.Exec(
				`INSERT OR REPLACE INTO ccusage_cache (key, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
				"daily", string(data),
			)
			if err != nil {
				slog.Debug("billing_cache: failed to persist daily", "error", err)
			}
		}
	}

	if block != nil {
		if data, err := json.Marshal(block); err == nil {
			_, err := c.db.Exec(
				`INSERT OR REPLACE INTO ccusage_cache (key, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
				"block", string(data),
			)
			if err != nil {
				slog.Debug("billing_cache: failed to persist block", "error", err)
			}
		}
	}
}

func (c *BillingCache) refresh() {
	daily := QueryDaily(c.ccusageCmd)
	block := QueryActiveBlock(c.ccusageCmd)

	// Only update cache entries that succeeded — don't overwrite good
	// DB-loaded or previously cached data with nil on ccusage failure.
	c.mu.Lock()
	if daily != nil {
		c.daily = daily
	}
	if block != nil {
		c.block = block
	}
	c.mu.Unlock()

	c.persistToDB(daily, block)
	slog.Debug("billing_cache: refreshed", "has_daily", daily != nil, "has_block", block != nil)
}

// ModelBreakdown holds per-model token and cost data.
type ModelBreakdown struct {
	ModelName           string  `json:"modelName"`
	InputTokens         int     `json:"inputTokens"`
	OutputTokens        int     `json:"outputTokens"`
	CacheCreationTokens int     `json:"cacheCreationTokens"`
	CacheReadTokens     int     `json:"cacheReadTokens"`
	Cost                float64 `json:"cost"`
}

// DailyEntry represents a single day from `ccusage daily --json --breakdown`.
type DailyEntry struct {
	Date                string           `json:"date"`
	InputTokens         int              `json:"inputTokens"`
	OutputTokens        int              `json:"outputTokens"`
	CacheCreationTokens int              `json:"cacheCreationTokens"`
	CacheReadTokens     int              `json:"cacheReadTokens"`
	TotalTokens         int              `json:"totalTokens"`
	TotalCost           float64          `json:"totalCost"`
	ModelsUsed          []string         `json:"modelsUsed,omitempty"`
	ModelBreakdowns     []ModelBreakdown `json:"modelBreakdowns,omitempty"`
}

// Totals holds aggregated totals across all entries.
type Totals struct {
	InputTokens         int     `json:"inputTokens"`
	OutputTokens        int     `json:"outputTokens"`
	CacheCreationTokens int     `json:"cacheCreationTokens"`
	CacheReadTokens     int     `json:"cacheReadTokens"`
	TotalTokens         int     `json:"totalTokens"`
	TotalCost           float64 `json:"totalCost"`
}

// DailyResponse is the top-level JSON from `ccusage daily --json`.
type DailyResponse struct {
	Daily  []DailyEntry `json:"daily"`
	Totals Totals       `json:"totals"`
}

// BlockEntry represents a 5-hour billing block from `ccusage blocks --json`.
type BlockEntry struct {
	BlockStart          string  `json:"blockStart"`
	BlockEnd            string  `json:"blockEnd"`
	InputTokens         int     `json:"inputTokens"`
	OutputTokens        int     `json:"outputTokens"`
	CacheCreationTokens int     `json:"cacheCreationTokens"`
	CacheReadTokens     int     `json:"cacheReadTokens"`
	TotalTokens         int     `json:"totalTokens"`
	TotalCost           float64 `json:"totalCost"`
	SessionCount        int     `json:"sessionCount"`
	IsActive            bool    `json:"isActive"`
	ProjectedTokens     int     `json:"projectedTokens,omitempty"`
	ProjectedCost       float64 `json:"projectedCost,omitempty"`
}

// BlockResponse is the top-level JSON from `ccusage blocks --json`.
type BlockResponse struct {
	Blocks []BlockEntry `json:"blocks"`
}

// run executes a ccusage command with a timeout and returns the raw JSON output.
func run(ccusageCmd string, args ...string) ([]byte, error) {
	parts := strings.Fields(ccusageCmd)
	if len(parts) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	fullArgs := append(parts[1:], args...)
	cmd := exec.CommandContext(ctx, parts[0], fullArgs...)
	return cmd.Output()
}

// QueryDaily returns today's usage from `ccusage daily --json --breakdown --since today`.
func QueryDaily(ccusageCmd string) *DailyEntry {
	today := time.Now().Format("20060102")
	output, err := run(ccusageCmd, "daily", "--json", "--breakdown", "--offline", "--since", today)
	if err != nil {
		slog.Warn("ccusage daily query failed", "error", err)
		return nil
	}

	var resp DailyResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		slog.Debug("ccusage daily parse failed", "error", err)
		return nil
	}
	if len(resp.Daily) == 0 {
		return &DailyEntry{Date: today}
	}
	return &resp.Daily[len(resp.Daily)-1]
}

// QueryActiveBlock returns the active 5-hour billing block from `ccusage blocks --active --json`.
func QueryActiveBlock(ccusageCmd string) *BlockEntry {
	output, err := run(ccusageCmd, "blocks", "--active", "--json", "--offline")
	if err != nil {
		slog.Warn("ccusage blocks query failed", "error", err)
		return nil
	}

	var resp BlockResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		slog.Debug("ccusage blocks parse failed", "error", err)
		return nil
	}
	for i := range resp.Blocks {
		if resp.Blocks[i].IsActive {
			return &resp.Blocks[i]
		}
	}
	if len(resp.Blocks) > 0 {
		return &resp.Blocks[len(resp.Blocks)-1]
	}
	return nil
}

// SessionEntry represents a single session from `ccusage session --json --id <id>`.
type SessionEntry struct {
	SessionID           string  `json:"sessionId"`
	InputTokens         int     `json:"inputTokens"`
	OutputTokens        int     `json:"outputTokens"`
	CacheCreationTokens int     `json:"cacheCreationTokens"`
	CacheReadTokens     int     `json:"cacheReadTokens"`
	TotalTokens         int     `json:"totalTokens"`
	TotalCost           float64 `json:"totalCost"`
}

// SessionResponse is the top-level JSON from `ccusage session --json`.
type SessionResponse struct {
	Sessions []SessionEntry `json:"sessions"`
	Totals   Totals         `json:"totals"`
}

// QuerySession returns usage for a specific session ID.
func QuerySession(ccusageCmd, sessionID string) *Totals {
	output, err := run(ccusageCmd, "session", "--id", sessionID, "--json", "--offline")
	if err != nil {
		slog.Warn("ccusage session query failed", "error", err, "session_id", sessionID)
		return nil
	}

	var resp SessionResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		slog.Debug("ccusage session parse failed", "error", err)
		return nil
	}

	if resp.Totals.TotalTokens > 0 {
		return &resp.Totals
	}

	// Fallback: sum session entries
	var totals Totals
	for _, s := range resp.Sessions {
		totals.TotalTokens += s.TotalTokens
		totals.TotalCost += s.TotalCost
		totals.InputTokens += s.InputTokens
		totals.OutputTokens += s.OutputTokens
		totals.CacheCreationTokens += s.CacheCreationTokens
		totals.CacheReadTokens += s.CacheReadTokens
	}
	return &totals
}

// QueryProjectUsage returns usage for a specific project (worktree).
// Used after task execution to get per-task token/cost data.
func QueryProjectUsage(ccusageCmd, projectName string) *Totals {
	output, err := run(ccusageCmd, "daily", "--project", projectName, "--json", "--offline")
	if err != nil {
		slog.Warn("ccusage project query failed", "error", err, "project", projectName)
		return nil
	}

	var resp DailyResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		slog.Debug("ccusage project parse failed", "error", err)
		return nil
	}

	if resp.Totals.TotalTokens > 0 {
		return &resp.Totals
	}

	// Fallback: sum daily entries if totals is empty
	var totals Totals
	for _, day := range resp.Daily {
		totals.TotalTokens += day.TotalTokens
		totals.TotalCost += day.TotalCost
		totals.InputTokens += day.InputTokens
		totals.OutputTokens += day.OutputTokens
		totals.CacheCreationTokens += day.CacheCreationTokens
		totals.CacheReadTokens += day.CacheReadTokens
	}
	return &totals
}
