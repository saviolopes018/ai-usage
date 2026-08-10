package codex

import (
	"encoding/json"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

type RateLimitWindow struct {
	UsedPercent        int    `json:"usedPercent"`
	WindowDurationMins *int64 `json:"windowDurationMins"`
	ResetsAt           *int64 `json:"resetsAt"`
}

type RateLimitSnapshot struct {
	Primary   *RateLimitWindow `json:"primary"`
	Secondary *RateLimitWindow `json:"secondary"`
}

type RateLimitsResponse struct {
	RateLimits          RateLimitSnapshot            `json:"rateLimits"`
	RateLimitsByLimitID map[string]RateLimitSnapshot `json:"rateLimitsByLimitId"`
}

func MapRateLimits(raw json.RawMessage, observedAt time.Time) (domain.ProviderUsage, error) {
	var response RateLimitsResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return domain.ProviderUsage{}, err
	}
	snapshot := response.RateLimits
	if codex, ok := response.RateLimitsByLimitID["codex"]; ok {
		snapshot = codex
	}
	usage := domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: observedAt.UTC()}
	for _, window := range []*RateLimitWindow{snapshot.Primary, snapshot.Secondary} {
		if window == nil || window.WindowDurationMins == nil {
			continue
		}
		mapped := mapWindow(window)
		switch *window.WindowDurationMins {
		case 300:
			usage.FiveHour = mapped
		case 10080:
			usage.Weekly = mapped
		}
	}
	return usage, nil
}

func mapWindow(window *RateLimitWindow) *domain.UsageWindow {
	used := float64(window.UsedPercent)
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	var reset time.Time
	if window.ResetsAt != nil {
		reset = time.Unix(*window.ResetsAt, 0).UTC()
	}
	return &domain.UsageWindow{UsedPercentage: used, RemainingPercentage: 100 - used, ResetsAt: reset}
}
