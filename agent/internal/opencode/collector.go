package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
}

type CommandRunner struct{ Binary string }

func (r CommandRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, r.Binary, args...).Output()
}

type SQLiteRunner struct {
	Binary   string
	Database string
}

func (r SQLiteRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("invalid OpenCode query arguments")
	}
	return exec.CommandContext(ctx, r.Binary, "-readonly", "-json", r.Database, args[3]).Output()
}

type Collector struct{ runners []Runner }

func NewCollector(runner Runner) Collector { return Collector{runners: []Runner{runner}} }

func NewFallbackCollector(primary, fallback Runner) Collector {
	return Collector{runners: []Runner{primary, fallback}}
}

type StateStore interface {
	Get() domain.UsageSnapshot
	UpdateProvider(domain.ProviderUsage) bool
}

type Tracker struct {
	collector Collector
	store     StateStore
}

func NewTracker(collector Collector, store StateStore) Tracker {
	return Tracker{collector: collector, store: store}
}

func (t Tracker) Refresh(ctx context.Context, now time.Time) error {
	usage, err := t.collector.Collect(ctx, now)
	if err == nil {
		t.store.UpdateProvider(usage)
		return nil
	}
	for _, provider := range t.store.Get().Providers {
		if provider.Provider == "opencode" {
			provider.Available = false
			t.store.UpdateProvider(provider)
			break
		}
	}
	return err
}

func ResolveBinary(home string) (string, error) {
	if binary, err := exec.LookPath("opencode"); err == nil {
		return binary, nil
	}
	for _, binary := range []string{
		filepath.Join(home, ".opencode", "bin", "opencode"),
		filepath.Join(home, ".local", "bin", "opencode"),
		"/opt/homebrew/bin/opencode",
		"/usr/local/bin/opencode",
	} {
		info, err := os.Stat(binary)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return binary, nil
		}
	}
	return "", fmt.Errorf("OpenCode executable not found")
}

func NewLocalCollector(home string) (Collector, string, error) {
	database := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	var fallback *SQLiteRunner
	if info, err := os.Stat(database); err == nil && info.Mode().IsRegular() {
		const sqlite = "/usr/bin/sqlite3"
		if sqliteInfo, sqliteErr := os.Stat(sqlite); sqliteErr == nil && sqliteInfo.Mode().IsRegular() && sqliteInfo.Mode().Perm()&0111 != 0 {
			fallback = &SQLiteRunner{Binary: sqlite, Database: database}
		}
	}
	if binary, err := ResolveBinary(home); err == nil {
		primary := CommandRunner{Binary: binary}
		if fallback != nil {
			return NewFallbackCollector(primary, *fallback), binary + " (fallback: " + database + ")", nil
		}
		return NewCollector(primary), binary, nil
	}
	if fallback != nil {
		return NewCollector(*fallback), database, nil
	}
	return Collector{}, "", fmt.Errorf("OpenCode CLI and database not found")
}

type aggregateRow struct {
	Input24h      int64 `json:"input_24h"`
	Output24h     int64 `json:"output_24h"`
	Reasoning24h  int64 `json:"reasoning_24h"`
	CacheRead24h  int64 `json:"cache_read_24h"`
	CacheWrite24h int64 `json:"cache_write_24h"`
	Input7d       int64 `json:"input_7d"`
	Output7d      int64 `json:"output_7d"`
	Reasoning7d   int64 `json:"reasoning_7d"`
	CacheRead7d   int64 `json:"cache_read_7d"`
	CacheWrite7d  int64 `json:"cache_write_7d"`
	Input14d      int64 `json:"input_14d"`
	Output14d     int64 `json:"output_14d"`
	Reasoning14d  int64 `json:"reasoning_14d"`
	CacheRead14d  int64 `json:"cache_read_14d"`
	CacheWrite14d int64 `json:"cache_write_14d"`
	Input30d      int64 `json:"input_30d"`
	Output30d     int64 `json:"output_30d"`
	Reasoning30d  int64 `json:"reasoning_30d"`
	CacheRead30d  int64 `json:"cache_read_30d"`
	CacheWrite30d int64 `json:"cache_write_30d"`
}

func (c Collector) Collect(ctx context.Context, now time.Time) (domain.ProviderUsage, error) {
	query := aggregateQuery(now)
	var row aggregateRow
	var attempts []error
	valid := false
	for _, runner := range c.runners {
		output, err := runner.Run(ctx, "db", "--format", "json", query)
		if err != nil {
			attempts = append(attempts, fmt.Errorf("query OpenCode usage: %w", err))
			continue
		}
		row, err = decodeAggregate(output)
		if err != nil {
			attempts = append(attempts, err)
			continue
		}
		valid = true
		break
	}
	if !valid {
		return domain.ProviderUsage{}, errors.Join(attempts...)
	}
	periods := map[string]domain.TokenUsage{
		"24h": tokenUsage(row.Input24h, row.Output24h, row.Reasoning24h, row.CacheRead24h, row.CacheWrite24h),
		"7d":  tokenUsage(row.Input7d, row.Output7d, row.Reasoning7d, row.CacheRead7d, row.CacheWrite7d),
		"14d": tokenUsage(row.Input14d, row.Output14d, row.Reasoning14d, row.CacheRead14d, row.CacheWrite14d),
		"30d": tokenUsage(row.Input30d, row.Output30d, row.Reasoning30d, row.CacheRead30d, row.CacheWrite30d),
	}
	tokens := periods["24h"]
	tokens.Periods = periods
	return domain.ProviderUsage{Provider: "opencode", Available: true, ObservedAt: now.UTC(), Tokens: &tokens}, nil
}

func decodeAggregate(output []byte) (aggregateRow, error) {
	var rows []aggregateRow
	if err := json.Unmarshal(output, &rows); err != nil {
		return aggregateRow{}, fmt.Errorf("decode OpenCode usage: %w", err)
	}
	if len(rows) != 1 {
		return aggregateRow{}, fmt.Errorf("decode OpenCode usage: expected one row, got %d", len(rows))
	}
	var rawRows []map[string]json.RawMessage
	if err := json.Unmarshal(output, &rawRows); err != nil || len(rawRows) != 1 {
		return aggregateRow{}, fmt.Errorf("decode OpenCode usage: invalid row")
	}
	for _, period := range []string{"24h", "7d", "14d", "30d"} {
		for _, metric := range []string{"input", "output", "reasoning", "cache_read", "cache_write"} {
			field := metric + "_" + period
			raw, exists := rawRows[0][field]
			var value int64
			if !exists || string(raw) == "null" || json.Unmarshal(raw, &value) != nil {
				return aggregateRow{}, fmt.Errorf("decode OpenCode usage: missing or invalid %s", field)
			}
		}
	}
	row := rows[0]
	values := []int64{
		row.Input24h, row.Output24h, row.Reasoning24h, row.CacheRead24h, row.CacheWrite24h,
		row.Input7d, row.Output7d, row.Reasoning7d, row.CacheRead7d, row.CacheWrite7d,
		row.Input14d, row.Output14d, row.Reasoning14d, row.CacheRead14d, row.CacheWrite14d,
		row.Input30d, row.Output30d, row.Reasoning30d, row.CacheRead30d, row.CacheWrite30d,
	}
	for _, value := range values {
		if value < 0 {
			return aggregateRow{}, fmt.Errorf("decode OpenCode usage: negative token count")
		}
	}
	return row, nil
}

func tokenUsage(input, output, reasoning, cacheRead, cacheWrite int64) domain.TokenUsage {
	input += cacheRead + cacheWrite
	output += reasoning
	return domain.TokenUsage{InputTokens: input, OutputTokens: output, CachedInputTokens: cacheRead, TotalTokens: input + output}
}

func aggregateQuery(now time.Time) string {
	cutoffs := map[string]int64{
		"24h": now.Add(-24 * time.Hour).UnixMilli(),
		"7d":  now.Add(-7 * 24 * time.Hour).UnixMilli(),
		"14d": now.Add(-14 * 24 * time.Hour).UnixMilli(),
		"30d": now.Add(-30 * 24 * time.Hour).UnixMilli(),
	}
	metric := func(period, path, alias string) string {
		return fmt.Sprintf("COALESCE(SUM(CASE WHEN time_created >= %d THEN COALESCE(json_extract(data, '%s'), 0) ELSE 0 END), 0) AS %s_%s", cutoffs[period], path, alias, period)
	}
	metrics := ""
	for _, period := range []string{"24h", "7d", "14d", "30d"} {
		for _, field := range [][2]string{{"$.tokens.input", "input"}, {"$.tokens.output", "output"}, {"$.tokens.reasoning", "reasoning"}, {"$.tokens.cache.read", "cache_read"}, {"$.tokens.cache.write", "cache_write"}} {
			if metrics != "" {
				metrics += ", "
			}
			metrics += metric(period, field[0], field[1])
		}
	}
	return fmt.Sprintf("SELECT %s FROM message WHERE json_extract(data, '$.role') = 'assistant' AND time_created >= %d", metrics, cutoffs["30d"])
}
