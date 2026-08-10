package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSnapshotSerialization(t *testing.T) {
	reset := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	s := UsageSnapshot{Device: "mac", Online: true, UpdatedAt: reset, Providers: []ProviderUsage{{Provider: "codex", Available: true, ObservedAt: reset, FiveHour: &UsageWindow{UsedPercentage: 25, RemainingPercentage: 75, ResetsAt: reset}}}}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(b)
	for _, field := range []string{`"updatedAt"`, `"fiveHour"`, `"usedPercentage":25`, `"remainingPercentage":75`} {
		if !strings.Contains(jsonText, field) {
			t.Errorf("missing %s in %s", field, jsonText)
		}
	}
	var roundtrip UsageSnapshot
	if err := json.Unmarshal(b, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if roundtrip.Providers[0].FiveHour.ResetsAt != reset {
		t.Fatalf("reset mismatch: %v", roundtrip)
	}
}

func TestInitialSnapshot(t *testing.T) {
	s := InitialSnapshot()
	if !s.Online || len(s.Providers) != 2 {
		t.Fatalf("unexpected initial snapshot: %+v", s)
	}
	for _, p := range s.Providers {
		if p.Available {
			t.Fatalf("%s should be unavailable", p.Provider)
		}
	}
}
