package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

const cacheFilename = "claude-usage.json"

func DefaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ai-usage", cacheFilename), nil
}

func LoadCache(path string) (domain.ProviderUsage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.ProviderUsage{}, err
	}
	var usage domain.ProviderUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return domain.ProviderUsage{}, fmt.Errorf("decode Claude cache: %w", err)
	}
	if usage.Provider != "claude" || !usage.Available || usage.ObservedAt.IsZero() || (usage.FiveHour == nil && usage.Weekly == nil) {
		return domain.ProviderUsage{}, errorsNewInvalidCache()
	}
	return usage, nil
}

func SaveCache(path string, usage domain.ProviderUsage) error {
	if usage.Provider != "claude" || !usage.Available || usage.ObservedAt.IsZero() || (usage.FiveHour == nil && usage.Weekly == nil) {
		return errorsNewInvalidCache()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(usage, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".claude-usage-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func errorsNewInvalidCache() error { return fmt.Errorf("invalid Claude cache") }
