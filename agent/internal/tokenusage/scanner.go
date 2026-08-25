package tokenusage

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

// Scan reads the local session histories maintained by Codex and Claude Code.
// It never reads provider credentials or sends session contents anywhere.
func Scan(home string) (map[string]domain.TokenUsage, error) {
	return ScanAt(home, time.Now())
}

func ScanAt(home string, now time.Time) (map[string]domain.TokenUsage, error) {
	cutoffs := map[string]time.Time{
		"24h": now.Add(-24 * time.Hour),
		"7d":  now.Add(-7 * 24 * time.Hour),
		"14d": now.Add(-14 * 24 * time.Hour),
		"30d": now.Add(-30 * 24 * time.Hour),
	}
	result := map[string]domain.TokenUsage{}
	var errs []error
	if usage, err := scanCodex(filepath.Join(home, ".codex", "sessions"), cutoffs); err == nil {
		result["codex"] = usageWithPeriods(usage)
	} else if !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}
	if usage, err := scanClaude(filepath.Join(home, ".claude", "projects"), cutoffs); err == nil {
		result["claude"] = usageWithPeriods(usage)
	} else if !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}
	return result, errors.Join(errs...)
}

func usageWithPeriods(periods map[string]domain.TokenUsage) domain.TokenUsage {
	usage := periods["24h"]
	usage.Periods = periods
	return usage
}

func jsonlFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".jsonl" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func scanCodex(root string, cutoffs map[string]time.Time) (map[string]domain.TokenUsage, error) {
	files, err := jsonlFiles(root)
	if err != nil {
		return nil, err
	}
	totals := make(map[string]domain.TokenUsage, len(cutoffs))
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		var previous domain.TokenUsage
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			var record struct {
				Timestamp time.Time `json:"timestamp"`
				Payload   struct {
					Info struct {
						Total struct {
							Input  int64 `json:"input_tokens"`
							Cached int64 `json:"cached_input_tokens"`
							Output int64 `json:"output_tokens"`
							Total  int64 `json:"total_tokens"`
						} `json:"total_token_usage"`
					} `json:"info"`
				} `json:"payload"`
			}
			if json.Unmarshal(scanner.Bytes(), &record) == nil && record.Payload.Info.Total.Total > 0 {
				value := record.Payload.Info.Total
				current := domain.TokenUsage{InputTokens: value.Input, CachedInputTokens: value.Cached, OutputTokens: value.Output, TotalTokens: value.Total}
				for period, cutoff := range cutoffs {
					if !record.Timestamp.Before(cutoff) {
						total := totals[period]
						total.InputTokens += positiveDelta(current.InputTokens, previous.InputTokens)
						total.CachedInputTokens += positiveDelta(current.CachedInputTokens, previous.CachedInputTokens)
						total.OutputTokens += positiveDelta(current.OutputTokens, previous.OutputTokens)
						total.TotalTokens += positiveDelta(current.TotalTokens, previous.TotalTokens)
						totals[period] = total
					}
				}
				previous = current
			}
		}
		_ = file.Close()
	}
	return totals, nil
}

func positiveDelta(current, previous int64) int64 {
	if current < previous {
		return current
	}
	return current - previous
}

func scanClaude(root string, cutoffs map[string]time.Time) (map[string]domain.TokenUsage, error) {
	files, err := jsonlFiles(root)
	if err != nil {
		return nil, err
	}
	messages := make(map[string]map[string]domain.TokenUsage, len(cutoffs))
	for period := range cutoffs {
		messages[period] = map[string]domain.TokenUsage{}
	}
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			var record struct {
				Timestamp time.Time `json:"timestamp"`
				Message   struct {
					ID    string `json:"id"`
					Usage struct {
						Input         int64 `json:"input_tokens"`
						Output        int64 `json:"output_tokens"`
						CacheCreation int64 `json:"cache_creation_input_tokens"`
						CacheRead     int64 `json:"cache_read_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(scanner.Bytes(), &record) != nil || record.Message.ID == "" {
				continue
			}
			usage := record.Message.Usage
			if usage.Input == 0 && usage.Output == 0 && usage.CacheCreation == 0 && usage.CacheRead == 0 {
				continue
			}
			input := usage.Input + usage.CacheCreation + usage.CacheRead
			current := domain.TokenUsage{InputTokens: input, CachedInputTokens: usage.CacheRead, OutputTokens: usage.Output, TotalTokens: input + usage.Output}
			for period, cutoff := range cutoffs {
				if record.Timestamp.Before(cutoff) {
					continue
				}
				if previous, exists := messages[period][record.Message.ID]; !exists || current.TotalTokens > previous.TotalTokens {
					messages[period][record.Message.ID] = current
				}
			}
		}
		_ = file.Close()
	}
	totals := make(map[string]domain.TokenUsage, len(cutoffs))
	for period, periodMessages := range messages {
		var total domain.TokenUsage
		for _, usage := range periodMessages {
			total.InputTokens += usage.InputTokens
			total.CachedInputTokens += usage.CachedInputTokens
			total.OutputTokens += usage.OutputTokens
			total.TotalTokens += usage.TotalTokens
		}
		totals[period] = total
	}
	return totals, nil
}
