package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

var (
	sessionUsagePattern = regexp.MustCompile(`(?m)^Current session:\s+([0-9]+(?:\.[0-9]+)?)% used\s+·\s+resets\s+(.+)$`)
	weeklyUsagePattern  = regexp.MustCompile(`(?m)^Current week(?: \([^)]*\))?:\s+([0-9]+(?:\.[0-9]+)?)% used\s+·\s+resets\s+(.+)$`)
	resetZonePattern    = regexp.MustCompile(`^(.+?)\s+\(([^)]+)\)$`)
)

type Refresher struct{ Binary string }

func (r Refresher) Refresh(ctx context.Context, now time.Time) (domain.ProviderUsage, error) {
	cmd := exec.CommandContext(ctx, r.Binary, "-p", "/usage", "--output-format", "json", "--no-session-persistence", "--tools", "", "--max-budget-usd", "0.000001")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return domain.ProviderUsage{}, fmt.Errorf("Claude usage command failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return domain.ProviderUsage{}, err
	}
	return ParseRefreshOutput(output, now)
}

func ParseRefreshOutput(data []byte, now time.Time) (domain.ProviderUsage, error) {
	var response struct {
		IsError      bool    `json:"is_error"`
		Result       string  `json:"result"`
		TotalCostUSD float64 `json:"total_cost_usd"`
		NumTurns     int     `json:"num_turns"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return domain.ProviderUsage{}, fmt.Errorf("decode Claude usage output: %w", err)
	}
	if response.IsError {
		return domain.ProviderUsage{}, errors.New("Claude returned an error while reading usage")
	}
	if response.TotalCostUSD != 0 || response.NumTurns != 0 {
		return domain.ProviderUsage{}, fmt.Errorf("refusing non-local Claude usage result: turns=%d cost=%g", response.NumTurns, response.TotalCostUSD)
	}
	usage := domain.ProviderUsage{Provider: "claude", Available: true, ObservedAt: now.UTC()}
	var err error
	if match := sessionUsagePattern.FindStringSubmatch(response.Result); match != nil {
		usage.FiveHour, err = parseRefreshWindow(match[1], match[2], now)
		if err != nil {
			return domain.ProviderUsage{}, fmt.Errorf("current session: %w", err)
		}
	}
	if match := weeklyUsagePattern.FindStringSubmatch(response.Result); match != nil {
		usage.Weekly, err = parseRefreshWindow(match[1], match[2], now)
		if err != nil {
			return domain.ProviderUsage{}, fmt.Errorf("current week: %w", err)
		}
	}
	if usage.FiveHour == nil && usage.Weekly == nil {
		return domain.ProviderUsage{}, errors.New("Claude usage output contains no supported limits")
	}
	return usage, nil
}

func parseRefreshWindow(percentText, resetText string, now time.Time) (*domain.UsageWindow, error) {
	used, err := strconv.ParseFloat(percentText, 64)
	if err != nil || used < 0 || used > 100 {
		return nil, errors.New("invalid percentage")
	}
	reset, err := parseRefreshReset(strings.TrimSpace(resetText), now)
	if err != nil {
		return nil, err
	}
	return &domain.UsageWindow{UsedPercentage: used, RemainingPercentage: 100 - used, ResetsAt: reset.UTC()}, nil
}

func parseRefreshReset(value string, now time.Time) (time.Time, error) {
	location := now.Location()
	if match := resetZonePattern.FindStringSubmatch(value); match != nil {
		value = strings.TrimSpace(match[1])
		loaded, err := time.LoadLocation(match[2])
		if err != nil {
			return time.Time{}, fmt.Errorf("unknown reset timezone: %w", err)
		}
		location = loaded
	}
	withYear := value + fmt.Sprintf(" %d", now.In(location).Year())
	var parsed time.Time
	var err error
	for _, layout := range []string{"Jan 2 at 3:04pm 2006", "Jan 2 at 3pm 2006"} {
		parsed, err = time.ParseInLocation(layout, withYear, location)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("parse reset: %w", err)
	}
	if parsed.Before(now.In(location).Add(-time.Minute)) {
		parsed = parsed.AddDate(1, 0, 0)
	}
	return parsed, nil
}
