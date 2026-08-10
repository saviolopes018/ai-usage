package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

func TestCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "claude-usage.json")
	want := domain.ProviderUsage{
		Provider: "claude", Available: true, ObservedAt: time.Date(2026, 8, 8, 20, 0, 0, 0, time.UTC),
		Weekly: &domain.UsageWindow{UsedPercentage: 24, RemainingPercentage: 76, ResetsAt: time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC)},
	}
	if err := SaveCache(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Weekly == nil || got.Weekly.UsedPercentage != 24 || !got.ObservedAt.Equal(want.ObservedAt) {
		t.Fatalf("got=%+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions=%o", info.Mode().Perm())
	}
}

func TestCacheRejectsCorruptAndUnavailableData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude-usage.json")
	if err := os.WriteFile(path, []byte(`{"provider":"claude"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCache(path); err == nil {
		t.Fatal("invalid cache accepted")
	}
	if err := SaveCache(path, domain.ProviderUsage{Provider: "claude"}); err == nil {
		t.Fatal("invalid usage saved")
	}
}
