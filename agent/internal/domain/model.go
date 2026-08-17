package domain

import (
	"os"
	"strings"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/protocol"
)

type UsageSnapshot struct {
	ProtocolVersion int             `json:"protocolVersion"`
	AgentVersion    string          `json:"agentVersion"`
	Capabilities    []string        `json:"capabilities"`
	Device          string          `json:"device"`
	Online          bool            `json:"online"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	Providers       []ProviderUsage `json:"providers"`
}

type ProviderUsage struct {
	Provider   string       `json:"provider"`
	Available  bool         `json:"available"`
	ObservedAt time.Time    `json:"observedAt"`
	FiveHour   *UsageWindow `json:"fiveHour,omitempty"`
	Weekly     *UsageWindow `json:"weekly,omitempty"`
	Tokens     *TokenUsage  `json:"tokens,omitempty"`
}

type TokenUsage struct {
	InputTokens       int64 `json:"inputTokens"`
	OutputTokens      int64 `json:"outputTokens"`
	CachedInputTokens int64 `json:"cachedInputTokens"`
	TotalTokens       int64 `json:"totalTokens"`
}

type UsageWindow struct {
	UsedPercentage      float64   `json:"usedPercentage"`
	RemainingPercentage float64   `json:"remainingPercentage"`
	ResetsAt            time.Time `json:"resetsAt"`
}

func InitialSnapshot() UsageSnapshot {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown"
	}
	now := time.Now().UTC()
	return UsageSnapshot{ProtocolVersion: protocol.Version, AgentVersion: protocol.AgentVersion, Capabilities: append([]string(nil), protocol.Capabilities...), Device: host, Online: true, UpdatedAt: now, Providers: []ProviderUsage{
		{Provider: "codex", Available: false, ObservedAt: now},
		{Provider: "claude", Available: false, ObservedAt: now},
	}}
}

func DisplayName(provider string) string {
	if provider == "codex" {
		return "Codex"
	}
	if provider == "claude" {
		return "Claude"
	}
	return provider
}
