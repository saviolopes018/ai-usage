package claude

import (
	"bytes"
	"testing"
	"time"
)

func TestParseComplete(t *testing.T) {
	usage, err := Parse([]byte(`{"rate_limits":{"five_hour":{"used_percentage":25,"resets_at":"2026-08-08T20:00:00Z"},"seven_day":{"used_percentage":40,"resets_at":1786819639}},"extra":true}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if usage.FiveHour == nil || usage.Weekly == nil || usage.FiveHour.RemainingPercentage != 75 {
		t.Fatalf("unexpected: %+v", usage)
	}
}
func TestParsePartial(t *testing.T) {
	for _, data := range []string{`{"rate_limits":{"five_hour":{"used_percentage":10}}}`, `{"rate_limits":{"seven_day":{"used_percentage":20}}}`} {
		usage, err := Parse([]byte(data), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if usage.FiveHour == nil && usage.Weekly == nil {
			t.Fatal("window missing")
		}
	}
}
func TestParseRejectsMissingNullAndInvalid(t *testing.T) {
	for _, data := range []string{`{}`, `{"rate_limits":null}`, `{"rate_limits":{}}`, `not json`} {
		if _, err := Parse([]byte(data), time.Now()); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}
func TestClamp(t *testing.T) {
	usage, err := Parse([]byte(`{"rate_limits":{"five_hour":{"used_percentage":-4},"seven_day":{"used_percentage":104}}}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if usage.FiveHour.UsedPercentage != 0 || usage.Weekly.UsedPercentage != 100 {
		t.Fatalf("not clamped: %+v", usage)
	}
}
func TestStatusLineSurvivesAgentOffline(t *testing.T) {
	var out bytes.Buffer
	if err := StatusLine([]byte(`{"rate_limits":{"five_hour":{"used_percentage":12}}}`), 1, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "AI usage: 5h 12%\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}
