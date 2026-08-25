package claude

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

func TestAutomaticRefresherPrefersOAuthAndFallsBackToCLI(t *testing.T) {
	now := time.Now()
	want := domain.ProviderUsage{Provider: "claude", Available: true, ObservedAt: now}
	cliCalls := 0
	refresher := AutomaticRefresher{
		OAuth: func(context.Context, time.Time) (domain.ProviderUsage, error) {
			return domain.ProviderUsage{}, errors.New("OAuth unavailable")
		},
		CLI: func(context.Context, time.Time) (domain.ProviderUsage, error) { cliCalls++; return want, nil },
	}
	got, err := refresher.Refresh(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != want.Provider || cliCalls != 1 {
		t.Fatalf("usage = %+v, cliCalls = %d", got, cliCalls)
	}

	refresher.OAuth = func(context.Context, time.Time) (domain.ProviderUsage, error) { return want, nil }
	cliCalls = 0
	if _, err := refresher.Refresh(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if cliCalls != 0 {
		t.Fatalf("CLI called after successful OAuth: %d", cliCalls)
	}
}

func TestAutomaticRefresherReportsBothErrors(t *testing.T) {
	refresher := AutomaticRefresher{
		OAuth: func(context.Context, time.Time) (domain.ProviderUsage, error) {
			return domain.ProviderUsage{}, errors.New("OAuth failed")
		},
		CLI: func(context.Context, time.Time) (domain.ProviderUsage, error) {
			return domain.ProviderUsage{}, errors.New("CLI failed")
		},
	}
	_, err := refresher.Refresh(context.Background(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "OAuth failed") || !strings.Contains(err.Error(), "CLI failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestOAuthRefresher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("anthropic-beta"); got != "oauth-2025-04-20" {
			t.Fatalf("anthropic-beta = %q", got)
		}
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":12.5,"resets_at":"2026-08-25T04:00:00Z"},"seven_day":{"utilization":31,"resets_at":"2026-08-26T13:00:00Z"}}`))
	}))
	defer server.Close()

	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	refresher := OAuthRefresher{Client: server.Client(), Endpoint: server.URL, AccessToken: func(context.Context) (string, error) { return "secret", nil }}
	usage, err := refresher.Refresh(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if usage.FiveHour == nil || usage.FiveHour.UsedPercentage != 12.5 || usage.Weekly == nil || usage.Weekly.RemainingPercentage != 69 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestOAuthRefresherDoesNotIncludeResponseBodyInHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"secret response"}`))
	}))
	defer server.Close()
	refresher := OAuthRefresher{Client: server.Client(), Endpoint: server.URL, AccessToken: func(context.Context) (string, error) { return "secret", nil }}
	_, err := refresher.Refresh(context.Background(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "secret response") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseOAuthUsageRejectsInvalidResponse(t *testing.T) {
	for _, data := range []string{`{}`, `{"five_hour":{"utilization":101}}`, `{"seven_day":{"utilization":10,"resets_at":"invalid"}}`} {
		if _, err := ParseOAuthUsage([]byte(data), time.Now()); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}
