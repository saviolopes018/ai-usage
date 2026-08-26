package store

import (
	"testing"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

func TestStoreReturnsCopies(t *testing.T) {
	s := New(domain.InitialSnapshot())
	got := s.Get()
	got.Providers[0].Available = true
	if s.Get().Providers[0].Available {
		t.Fatal("Get exposed internal state")
	}
}

func TestUpdateProviderPublishesChangedSnapshot(t *testing.T) {
	s := New(domain.InitialSnapshot())
	updates, unsubscribe := s.Subscribe()
	defer unsubscribe()
	p := domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: time.Now().UTC()}
	if !s.UpdateProvider(p) {
		t.Fatal("expected change")
	}
	select {
	case got := <-updates:
		if !got.Providers[0].Available {
			t.Fatal("update not stored")
		}
	case <-time.After(time.Second):
		t.Fatal("no update published")
	}
	if s.UpdateProvider(p) {
		t.Fatal("identical update should not publish")
	}
}

func TestSlowSubscriberReceivesLatestSnapshot(t *testing.T) {
	s := New(domain.InitialSnapshot())
	updates, unsubscribe := s.Subscribe()
	defer unsubscribe()
	for i := 1; i <= 20; i++ {
		s.UpdateProvider(domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: time.Unix(int64(i), 0).UTC()})
	}
	got := <-updates
	if got.Providers[0].ObservedAt != time.Unix(20, 0).UTC() {
		t.Fatalf("slow subscriber received stale state: %s", got.Providers[0].ObservedAt)
	}
}

func TestSubscribeWithInitialIsAtomic(t *testing.T) {
	s := New(domain.InitialSnapshot())
	updates, unsubscribe := s.SubscribeWithInitial()
	defer unsubscribe()
	initial := <-updates
	if len(initial.Providers) != 3 {
		t.Fatalf("unexpected initial snapshot: %+v", initial)
	}
	s.UpdateProvider(domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: time.Now().UTC()})
	if updated := <-updates; !updated.Providers[0].Available {
		t.Fatalf("missing update: %+v", updated)
	}
}

func TestRateLimitUpdatePreservesTokenUsage(t *testing.T) {
	s := New(domain.InitialSnapshot())
	tokens := &domain.TokenUsage{InputTokens: 100, OutputTokens: 20, TotalTokens: 120}
	s.UpdateProvider(domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: time.Unix(1, 0).UTC(), Tokens: tokens})
	s.UpdateProvider(domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: time.Unix(2, 0).UTC()})
	got := s.Get().Providers[0].Tokens
	if got == nil || got.TotalTokens != 120 {
		t.Fatalf("token usage was lost: %+v", got)
	}
}
