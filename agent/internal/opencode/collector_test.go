package opencode

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/store"
)

type recordingRunner struct {
	output []byte
	err    error
	args   []string
}

func (r *recordingRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	r.args = append([]string(nil), args...)
	return r.output, r.err
}

func TestCollectorMapsOpenCodeUsagePeriods(t *testing.T) {
	runner := &recordingRunner{output: []byte(`[{
		"input_24h":10,"output_24h":2,"reasoning_24h":3,"cache_read_24h":5,"cache_write_24h":7,
		"input_7d":20,"output_7d":4,"reasoning_7d":6,"cache_read_7d":10,"cache_write_7d":14,
		"input_14d":30,"output_14d":6,"reasoning_14d":9,"cache_read_14d":15,"cache_write_14d":21,
		"input_30d":40,"output_30d":8,"reasoning_30d":12,"cache_read_30d":20,"cache_write_30d":28
	}]`)}
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

	usage, err := NewCollector(runner).Collect(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}

	if usage.Provider != "opencode" || !usage.Available || !usage.ObservedAt.Equal(now) {
		t.Fatalf("unexpected provider state: %+v", usage)
	}
	if usage.Tokens == nil {
		t.Fatal("tokens should be present")
	}
	if got := *usage.Tokens; got.InputTokens != 22 || got.OutputTokens != 5 || got.CachedInputTokens != 5 || got.TotalTokens != 27 {
		t.Fatalf("unexpected 24h usage: %+v", got)
	}
	if got := usage.Tokens.Periods["30d"]; got.InputTokens != 88 || got.OutputTokens != 20 || got.CachedInputTokens != 20 || got.TotalTokens != 108 {
		t.Fatalf("unexpected 30d usage: %+v", got)
	}
	if len(runner.args) != 4 || runner.args[0] != "db" || runner.args[1] != "--format" || runner.args[2] != "json" {
		t.Fatalf("unexpected command arguments: %q", runner.args)
	}
	if query := runner.args[3]; !strings.Contains(query, "json_extract(data, '$.role') = 'assistant'") || !strings.Contains(query, "time_created >= 1785153600000") {
		t.Fatalf("query does not filter assistant messages by real 30d timestamp: %s", query)
	}
}

func TestCollectorRejectsMalformedResults(t *testing.T) {
	for _, output := range []string{`not-json`, `[]`, `[{}]`, `[{"input_24h":-1}]`} {
		t.Run(output, func(t *testing.T) {
			_, err := NewCollector(&recordingRunner{output: []byte(output)}).Collect(context.Background(), time.Now())
			if err == nil {
				t.Fatal("expected malformed result to fail")
			}
		})
	}
}

func TestCollectorPropagatesCommandFailure(t *testing.T) {
	want := errors.New("opencode unavailable")
	_, err := NewCollector(&recordingRunner{err: want}).Collect(context.Background(), time.Now())
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want wrapped command error", err)
	}
}

func TestCollectorFallsBackAfterCLIErrorOrMalformedOutput(t *testing.T) {
	valid := []byte(`[{"input_24h":10,"output_24h":2,"reasoning_24h":3,"cache_read_24h":5,"cache_write_24h":7,"input_7d":10,"output_7d":2,"reasoning_7d":3,"cache_read_7d":5,"cache_write_7d":7,"input_14d":10,"output_14d":2,"reasoning_14d":3,"cache_read_14d":5,"cache_write_14d":7,"input_30d":10,"output_30d":2,"reasoning_30d":3,"cache_read_30d":5,"cache_write_30d":7}]`)
	for _, primary := range []*recordingRunner{{err: errors.New("unsupported db command")}, {output: []byte("not-json")}} {
		t.Run(primary.errString(), func(t *testing.T) {
			fallback := &recordingRunner{output: valid}
			usage, err := NewFallbackCollector(primary, fallback).Collect(context.Background(), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if usage.Tokens == nil || usage.Tokens.TotalTokens != 27 {
				t.Fatalf("unexpected fallback usage: %+v", usage)
			}
			if len(fallback.args) == 0 {
				t.Fatal("fallback runner was not used")
			}
		})
	}
}

func (r *recordingRunner) errString() string {
	if r.err != nil {
		return "command-error"
	}
	return "malformed-output"
}

func TestResolveBinaryFindsOpenCodeInstallOutsidePath(t *testing.T) {
	home := t.TempDir()
	binary := filepath.Join(home, ".opencode", "bin", "opencode")
	if err := os.MkdirAll(filepath.Dir(binary), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())

	got, err := ResolveBinary(home)
	if err != nil {
		t.Fatal(err)
	}
	if got != binary {
		t.Fatalf("ResolveBinary() = %q, want %q", got, binary)
	}
}

func TestResolveBinaryRejectsMissingInstallation(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := ResolveBinary(t.TempDir()); err == nil {
		t.Fatal("expected missing OpenCode installation to fail")
	}
}

func TestTrackerMarksOnlyOpenCodeUnavailableAndPreservesTokensOnFailure(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	initial := domain.InitialSnapshot()
	tokens := domain.TokenUsage{TotalTokens: 42}
	initial.Providers[2] = domain.ProviderUsage{Provider: "opencode", Available: true, ObservedAt: now.Add(-time.Minute), Tokens: &tokens}
	state := store.New(initial)
	tracker := NewTracker(NewCollector(&recordingRunner{err: errors.New("offline")}), state)

	if err := tracker.Refresh(context.Background(), now); err == nil {
		t.Fatal("expected refresh failure")
	}

	got := state.Get()
	if got.Providers[0].Provider != "codex" || got.Providers[0].Available {
		t.Fatalf("Codex state changed unexpectedly: %+v", got.Providers[0])
	}
	if got.Providers[2].Available || got.Providers[2].Tokens == nil || got.Providers[2].Tokens.TotalTokens != 42 {
		t.Fatalf("unexpected OpenCode fallback state: %+v", got.Providers[2])
	}
}

func TestNewLocalCollectorFallsBackToDesktopDatabase(t *testing.T) {
	if _, err := os.Stat("/usr/bin/sqlite3"); err != nil {
		t.Skip("macOS sqlite3 is unavailable")
	}
	home := t.TempDir()
	database := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if err := os.MkdirAll(filepath.Dir(database), 0700); err != nil {
		t.Fatal(err)
	}
	create := `CREATE TABLE message (time_created integer NOT NULL, data text NOT NULL); INSERT INTO message VALUES (1787742000000, '{"role":"assistant","tokens":{"input":10,"output":2,"reasoning":3,"cache":{"read":5,"write":7}}}');`
	if output, err := exec.Command("/usr/bin/sqlite3", database, create).CombinedOutput(); err != nil {
		t.Fatalf("create fixture database: %v: %s", err, output)
	}
	t.Setenv("PATH", t.TempDir())

	collector, source, err := NewLocalCollector(home)
	if err != nil {
		t.Fatal(err)
	}
	if source != database {
		t.Fatalf("source = %q, want database path", source)
	}
	usage, err := collector.Collect(context.Background(), time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if usage.Tokens == nil || usage.Tokens.TotalTokens != 27 {
		t.Fatalf("unexpected desktop usage: %+v", usage)
	}
}
