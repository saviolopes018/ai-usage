package codex

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMapWeeklyOnly(t *testing.T) {
	raw := json.RawMessage(`{"rateLimits":{"primary":{"usedPercent":37,"windowDurationMins":10080,"resetsAt":1786819639},"secondary":null}}`)
	usage, err := MapRateLimits(raw, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if usage.FiveHour != nil || usage.Weekly == nil || usage.Weekly.UsedPercentage != 37 || usage.Weekly.RemainingPercentage != 63 {
		t.Fatalf("unexpected: %+v", usage)
	}
}
func TestMapFiveHourOnlyAndClamp(t *testing.T) {
	raw := json.RawMessage(`{"rateLimitsByLimitId":{"codex":{"primary":{"usedPercent":120,"windowDurationMins":300}}},"rateLimits":{}}`)
	usage, err := MapRateLimits(raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if usage.FiveHour == nil || usage.FiveHour.UsedPercentage != 100 || usage.Weekly != nil {
		t.Fatalf("unexpected: %+v", usage)
	}
}
func TestUnknownDurationIsNotInvented(t *testing.T) {
	usage, err := MapRateLimits(json.RawMessage(`{"rateLimits":{"primary":{"usedPercent":10,"windowDurationMins":60}}}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if usage.FiveHour != nil || usage.Weekly != nil {
		t.Fatalf("invented window: %+v", usage)
	}
}
func TestInvalidJSON(t *testing.T) {
	if _, err := MapRateLimits(json.RawMessage(`{`), time.Now()); err == nil {
		t.Fatal("expected error")
	}
}
