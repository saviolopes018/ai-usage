package store

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

type Store struct {
	mu          sync.RWMutex
	snapshot    domain.UsageSnapshot
	subscribers map[chan domain.UsageSnapshot]struct{}
}

func New(initial domain.UsageSnapshot) *Store {
	return &Store{snapshot: clone(initial), subscribers: make(map[chan domain.UsageSnapshot]struct{})}
}
func (s *Store) Get() domain.UsageSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.snapshot)
}
func (s *Store) UpdateProvider(provider domain.ProviderUsage) bool {
	s.mu.Lock()
	next := clone(s.snapshot)
	found := false
	for i := range next.Providers {
		if next.Providers[i].Provider == provider.Provider {
			next.Providers[i] = provider
			found = true
			break
		}
	}
	if !found {
		next.Providers = append(next.Providers, provider)
	}
	if equalIgnoringUpdatedAt(next, s.snapshot) {
		s.mu.Unlock()
		return false
	}
	next.UpdatedAt = time.Now().UTC()
	s.snapshot = next
	for ch := range s.subscribers {
		deliverLatest(ch, clone(next))
	}
	s.mu.Unlock()
	return true
}
func (s *Store) Subscribe() (<-chan domain.UsageSnapshot, func()) {
	ch := make(chan domain.UsageSnapshot, 1)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() { s.mu.Lock(); delete(s.subscribers, ch); s.mu.Unlock() }
}
func (s *Store) SubscribeWithInitial() (<-chan domain.UsageSnapshot, func()) {
	ch := make(chan domain.UsageSnapshot, 1)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	ch <- clone(s.snapshot)
	s.mu.Unlock()
	return ch, func() { s.mu.Lock(); delete(s.subscribers, ch); s.mu.Unlock() }
}
func deliverLatest(ch chan domain.UsageSnapshot, snapshot domain.UsageSnapshot) {
	select {
	case ch <- snapshot:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	ch <- snapshot
}
func clone(v domain.UsageSnapshot) domain.UsageSnapshot {
	b, _ := json.Marshal(v)
	var out domain.UsageSnapshot
	_ = json.Unmarshal(b, &out)
	return out
}
func equalIgnoringUpdatedAt(a, b domain.UsageSnapshot) bool {
	a.UpdatedAt = time.Time{}
	b.UpdatedAt = time.Time{}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
