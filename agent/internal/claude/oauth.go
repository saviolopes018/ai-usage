package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

const (
	DefaultOAuthUsageEndpoint = "https://api.anthropic.com/api/oauth/usage"
	keychainService           = "Claude Code-credentials"
)

type AccessTokenSource func(context.Context) (string, error)

type OAuthRefresher struct {
	Client      *http.Client
	Endpoint    string
	AccessToken AccessTokenSource
}

type RefreshFunc func(context.Context, time.Time) (domain.ProviderUsage, error)

type AutomaticRefresher struct {
	OAuth RefreshFunc
	CLI   RefreshFunc
}

func (r AutomaticRefresher) Refresh(ctx context.Context, now time.Time) (domain.ProviderUsage, error) {
	usage, oauthErr := r.OAuth(ctx, now)
	if oauthErr == nil {
		return usage, nil
	}
	usage, cliErr := r.CLI(ctx, now)
	if cliErr == nil {
		return usage, nil
	}
	return domain.ProviderUsage{}, fmt.Errorf("Claude usage unavailable: oauth: %v; cli: %v", oauthErr, cliErr)
}

func KeychainAccessToken(ctx context.Context) (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolve current user for Claude Keychain credential: %w", err)
	}
	output, err := exec.CommandContext(ctx, "security", "find-generic-password", "-s", keychainService, "-a", currentUser.Username, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("read Claude OAuth credential from Keychain: %w", err)
	}
	var credential struct {
		OAuth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(output, &credential); err != nil {
		return "", fmt.Errorf("decode Claude OAuth credential: %w", err)
	}
	token := strings.TrimSpace(credential.OAuth.AccessToken)
	if token == "" {
		return "", errors.New("Claude Code OAuth access token missing from Keychain; run `claude auth login`")
	}
	return token, nil
}

func (r OAuthRefresher) Refresh(ctx context.Context, now time.Time) (domain.ProviderUsage, error) {
	tokenSource := r.AccessToken
	if tokenSource == nil {
		tokenSource = KeychainAccessToken
	}
	token, err := tokenSource(ctx)
	if err != nil {
		return domain.ProviderUsage{}, err
	}
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = DefaultOAuthUsageEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.ProviderUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return domain.ProviderUsage{}, fmt.Errorf("request Claude OAuth usage: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxPayload+1))
	if err != nil {
		return domain.ProviderUsage{}, fmt.Errorf("read Claude OAuth usage: %w", err)
	}
	if len(body) > MaxPayload {
		return domain.ProviderUsage{}, errors.New("Claude OAuth usage response too large")
	}
	if resp.StatusCode != http.StatusOK {
		return domain.ProviderUsage{}, fmt.Errorf("Claude OAuth usage returned HTTP %d", resp.StatusCode)
	}
	return ParseOAuthUsage(body, now)
}

func ParseOAuthUsage(data []byte, observedAt time.Time) (domain.ProviderUsage, error) {
	var response struct {
		FiveHour *oauthWindow `json:"five_hour"`
		SevenDay *oauthWindow `json:"seven_day"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return domain.ProviderUsage{}, fmt.Errorf("decode Claude OAuth usage: %w", err)
	}
	usage := domain.ProviderUsage{Provider: "claude", Available: true, ObservedAt: observedAt.UTC()}
	var err error
	if response.FiveHour != nil {
		usage.FiveHour, err = response.FiveHour.mapWindow()
		if err != nil {
			return domain.ProviderUsage{}, fmt.Errorf("five_hour: %w", err)
		}
	}
	if response.SevenDay != nil {
		usage.Weekly, err = response.SevenDay.mapWindow()
		if err != nil {
			return domain.ProviderUsage{}, fmt.Errorf("seven_day: %w", err)
		}
	}
	if usage.FiveHour == nil && usage.Weekly == nil {
		return domain.ProviderUsage{}, errors.New("Claude OAuth usage contains no supported limits")
	}
	return usage, nil
}

type oauthWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

func (w oauthWindow) mapWindow() (*domain.UsageWindow, error) {
	if w.Utilization == nil || *w.Utilization < 0 || *w.Utilization > 100 {
		return nil, errors.New("invalid utilization")
	}
	var reset time.Time
	var err error
	if w.ResetsAt != "" {
		reset, err = time.Parse(time.RFC3339, w.ResetsAt)
		if err != nil {
			return nil, fmt.Errorf("parse resets_at: %w", err)
		}
	}
	return &domain.UsageWindow{UsedPercentage: *w.Utilization, RemainingPercentage: 100 - *w.Utilization, ResetsAt: reset.UTC()}, nil
}
