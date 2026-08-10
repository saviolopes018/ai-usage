package snapshotcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

const filename = "snapshot.json"

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ai-usage", filename), nil
}

func Load(path string) (domain.UsageSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.UsageSnapshot{}, err
	}
	var snapshot domain.UsageSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return domain.UsageSnapshot{}, fmt.Errorf("decode snapshot cache: %w", err)
	}
	if snapshot.UpdatedAt.IsZero() || len(snapshot.Providers) == 0 {
		return domain.UsageSnapshot{}, fmt.Errorf("invalid snapshot cache")
	}
	return snapshot, nil
}

func Save(path string, snapshot domain.UsageSnapshot) error {
	if snapshot.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid snapshot cache")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func Restore(fresh, cached domain.UsageSnapshot) domain.UsageSnapshot {
	fresh.UpdatedAt = cached.UpdatedAt
	for _, provider := range cached.Providers {
		if !provider.Available || provider.ObservedAt.IsZero() {
			continue
		}
		for i := range fresh.Providers {
			if fresh.Providers[i].Provider == provider.Provider {
				fresh.Providers[i] = provider
				break
			}
		}
	}
	fresh.Online = true
	return fresh
}

func IsStale(snapshot domain.UsageSnapshot, now time.Time) bool {
	return now.Sub(snapshot.UpdatedAt) > 15*time.Minute
}
