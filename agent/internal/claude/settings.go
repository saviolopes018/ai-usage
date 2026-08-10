package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ConfigureSettings(path, binary string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	settings := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("decode Claude settings: %w", err)
		}
	}
	if len(data) > 0 {
		backup := path + ".ai-usage-backup"
		f, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			if _, err = f.Write(data); err != nil {
				_ = f.Close()
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
		} else if !os.IsExist(err) {
			return err
		}
	}
	settings["statusLine"] = map[string]any{"type": "command", "command": shellQuote(binary) + " claude-statusline"}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings.json.ai-usage-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(out); err != nil {
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
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }
