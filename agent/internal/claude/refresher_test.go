package claude

import (
	"testing"
	"time"
)

func TestParseRefreshOutput(t *testing.T) {
	now := time.Date(2026, 8, 8, 20, 50, 0, 0, time.UTC)
	data := []byte(`{"is_error":false,"num_turns":0,"total_cost_usd":0,"result":"You are currently using your subscription\n\nCurrent session: 5% used · resets Aug 8 at 9:09pm (America/Fortaleza)\nCurrent week (all models): 24% used · resets Aug 12 at 9:59am (America/Fortaleza)"}`)
	got, err := ParseRefreshOutput(data, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.FiveHour == nil || got.Weekly == nil || got.FiveHour.UsedPercentage != 5 || got.Weekly.RemainingPercentage != 76 {
		t.Fatalf("got=%+v", got)
	}
	if got.FiveHour.ResetsAt.Format(time.RFC3339) != "2026-08-09T00:09:00Z" {
		t.Fatalf("reset=%s", got.FiveHour.ResetsAt)
	}
}

func TestParseRefreshOutputRejectsInferenceAndUnknownFormat(t *testing.T) {
	now := time.Now()
	for _, data := range []string{
		`{"is_error":false,"num_turns":1,"total_cost_usd":0,"result":"Current session: 5% used · resets Aug 8 at 9:09pm (UTC)"}`,
		`{"is_error":false,"num_turns":0,"total_cost_usd":0.01,"result":"Current session: 5% used · resets Aug 8 at 9:09pm (UTC)"}`,
		`{"is_error":false,"num_turns":0,"total_cost_usd":0,"result":"format changed"}`,
	} {
		if _, err := ParseRefreshOutput([]byte(data), now); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}

func TestParseRefreshOutputAcceptsWholeHourReset(t *testing.T) {
	now := time.Date(2026, 8, 8, 20, 50, 0, 0, time.UTC)
	data := []byte(`{"is_error":false,"num_turns":0,"total_cost_usd":0,"result":"Current week (all models): 24% used · resets Aug 12 at 10am (America/Fortaleza)"}`)
	got, err := ParseRefreshOutput(data, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Weekly == nil || got.Weekly.ResetsAt.Format(time.RFC3339) != "2026-08-12T13:00:00Z" {
		t.Fatalf("weekly=%+v", got.Weekly)
	}
}
