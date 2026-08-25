package tokenusage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanAggregatesCodexSessionsAndDeduplicatesClaudeMessages(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex", "sessions", "2026", "08")
	claudeDir := filepath.Join(home, ".claude", "projects", "project")
	if err := os.MkdirAll(codexDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 17, 18, 0, 0, 0, time.UTC)
	codex := `{"timestamp":"2026-08-16T17:00:00Z","payload":{"info":{"total_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":2,"total_tokens":12}}}}
{"timestamp":"2026-08-17T17:00:00Z","payload":{"info":{"total_token_usage":{"input_tokens":30,"cached_input_tokens":8,"output_tokens":5,"total_tokens":35}}}}
`
	claude := `{"timestamp":"2026-08-17T16:00:00Z","message":{"id":"msg-1","usage":{"input_tokens":3,"cache_creation_input_tokens":7,"cache_read_input_tokens":11,"output_tokens":5}}}
{"timestamp":"2026-08-17T16:00:01Z","message":{"id":"msg-1","usage":{"input_tokens":3,"cache_creation_input_tokens":7,"cache_read_input_tokens":11,"output_tokens":5}}}
{"timestamp":"2026-08-17T17:00:00Z","message":{"id":"msg-2","usage":{"input_tokens":2,"output_tokens":4}}}
{"timestamp":"2026-08-16T17:00:00Z","message":{"id":"old","usage":{"input_tokens":100,"output_tokens":100}}}
`
	if err := os.WriteFile(filepath.Join(codexDir, "session.jsonl"), []byte(codex), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "session.jsonl"), []byte(claude), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ScanAt(home, now)
	if err != nil {
		t.Fatal(err)
	}
	if got["codex"].TotalTokens != 23 || got["codex"].InputTokens != 20 || got["codex"].OutputTokens != 3 {
		t.Fatalf("unexpected Codex usage: %+v", got["codex"])
	}
	if got["claude"].TotalTokens != 32 || got["claude"].InputTokens != 23 || got["claude"].OutputTokens != 9 || got["claude"].CachedInputTokens != 11 {
		t.Fatalf("unexpected Claude usage: %+v", got["claude"])
	}
}

func TestScanSeparatesTokenUsageByPeriod(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex", "sessions")
	claudeDir := filepath.Join(home, ".claude", "projects")
	if err := os.MkdirAll(codexDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	codex := `{"timestamp":"2026-07-25T12:00:00Z","payload":{"info":{"total_token_usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11}}}}
{"timestamp":"2026-08-05T12:00:00Z","payload":{"info":{"total_token_usage":{"input_tokens":30,"output_tokens":3,"total_tokens":33}}}}
{"timestamp":"2026-08-15T12:00:00Z","payload":{"info":{"total_token_usage":{"input_tokens":60,"output_tokens":6,"total_tokens":66}}}}
{"timestamp":"2026-08-22T12:00:00Z","payload":{"info":{"total_token_usage":{"input_tokens":100,"output_tokens":10,"total_tokens":110}}}}
{"timestamp":"2026-08-25T11:00:00Z","payload":{"info":{"total_token_usage":{"input_tokens":150,"output_tokens":15,"total_tokens":165}}}}
`
	if err := os.WriteFile(filepath.Join(codexDir, "session.jsonl"), []byte(codex), 0600); err != nil {
		t.Fatal(err)
	}
	claude := `{"timestamp":"2026-07-25T12:00:00Z","message":{"id":"outside","usage":{"input_tokens":10,"output_tokens":1}}}
{"timestamp":"2026-08-05T12:00:00Z","message":{"id":"month","usage":{"input_tokens":10,"output_tokens":1}}}
{"timestamp":"2026-08-15T12:00:00Z","message":{"id":"fortnight","usage":{"input_tokens":10,"output_tokens":1}}}
{"timestamp":"2026-08-22T12:00:00Z","message":{"id":"week","usage":{"input_tokens":10,"output_tokens":1}}}
{"timestamp":"2026-08-25T11:00:00Z","message":{"id":"day","usage":{"input_tokens":10,"output_tokens":1}}}
`
	if err := os.WriteFile(filepath.Join(claudeDir, "session.jsonl"), []byte(claude), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ScanAt(home, now)
	if err != nil {
		t.Fatal(err)
	}

	periods := got["codex"].Periods
	for period, want := range map[string]int64{"24h": 55, "7d": 99, "14d": 132, "30d": 154} {
		if periods[period].TotalTokens != want {
			t.Errorf("%s total = %d, want %d", period, periods[period].TotalTokens, want)
		}
	}
	if got["codex"].TotalTokens != periods["24h"].TotalTokens {
		t.Errorf("legacy total = %d, want 24h total %d", got["codex"].TotalTokens, periods["24h"].TotalTokens)
	}
	for period, want := range map[string]int64{"24h": 11, "7d": 22, "14d": 33, "30d": 44} {
		if got["claude"].Periods[period].TotalTokens != want {
			t.Errorf("Claude %s total = %d, want %d", period, got["claude"].Periods[period].TotalTokens, want)
		}
	}
}
