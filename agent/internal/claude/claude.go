package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

const MaxPayload = 64 << 10

type rawWindow struct {
	UsedPercentage *float64        `json:"used_percentage"`
	ResetsAt       json.RawMessage `json:"resets_at"`
}
type payload struct {
	RateLimits *struct {
		FiveHour *rawWindow `json:"five_hour"`
		SevenDay *rawWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

func Parse(data []byte, observedAt time.Time) (domain.ProviderUsage, error) {
	if len(data) > MaxPayload {
		return domain.ProviderUsage{}, errors.New("payload too large")
	}
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return domain.ProviderUsage{}, err
	}
	if p.RateLimits == nil {
		return domain.ProviderUsage{}, errors.New("rate_limits missing")
	}
	usage := domain.ProviderUsage{Provider: "claude", Available: true, ObservedAt: observedAt.UTC()}
	var err error
	if p.RateLimits.FiveHour != nil {
		usage.FiveHour, err = mapRaw(p.RateLimits.FiveHour)
		if err != nil {
			return domain.ProviderUsage{}, fmt.Errorf("five_hour: %w", err)
		}
	}
	if p.RateLimits.SevenDay != nil {
		usage.Weekly, err = mapRaw(p.RateLimits.SevenDay)
		if err != nil {
			return domain.ProviderUsage{}, fmt.Errorf("seven_day: %w", err)
		}
	}
	if usage.FiveHour == nil && usage.Weekly == nil {
		return domain.ProviderUsage{}, errors.New("rate_limits has no valid windows")
	}
	return usage, nil
}
func mapRaw(raw *rawWindow) (*domain.UsageWindow, error) {
	if raw.UsedPercentage == nil {
		return nil, errors.New("used_percentage missing")
	}
	used := *raw.UsedPercentage
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	reset, err := parseReset(raw.ResetsAt)
	if err != nil {
		return nil, err
	}
	return &domain.UsageWindow{UsedPercentage: used, RemainingPercentage: 100 - used, ResetsAt: reset}, nil
}
func parseReset(raw json.RawMessage) (time.Time, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return time.Time{}, nil
	}
	var unix int64
	if json.Unmarshal(raw, &unix) == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return time.Time{}, errors.New("invalid resets_at")
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func StatusLine(data []byte, port int, stdout io.Writer) error {
	usage, err := Parse(data, time.Now())
	if err != nil {
		fmt.Fprintln(stdout, "AI usage: unavailable")
		return nil
	}
	label := "AI usage:"
	if usage.FiveHour != nil {
		label += fmt.Sprintf(" 5h %.0f%%", usage.FiveHour.UsedPercentage)
	}
	if usage.Weekly != nil {
		label += fmt.Sprintf(" 7d %.0f%%", usage.Weekly.UsedPercentage)
	}
	fmt.Fprintln(stdout, label)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/internal/claude/status", port), bytes.NewReader(data))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 750 * time.Millisecond}
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	return nil
}
