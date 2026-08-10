package snapshotcache

import (
	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTripAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	now := time.Now().UTC().Add(-time.Hour)
	cached := domain.InitialSnapshot()
	cached.UpdatedAt = now
	cached.Providers[0] = domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: now, Weekly: &domain.UsageWindow{UsedPercentage: 42, RemainingPercentage: 58}}
	if err := Save(path, cached); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	restored := Restore(domain.InitialSnapshot(), got)
	if !restored.Providers[0].Available || restored.Providers[0].Weekly.UsedPercentage != 42 || !IsStale(restored, time.Now()) {
		t.Fatalf("restored=%+v", restored)
	}
	if mode, _ := os.Stat(path); mode.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o", mode.Mode().Perm())
	}
}
