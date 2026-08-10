package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigureSettingsPreservesExistingAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"enableWorkflows":true,"theme":"dark"}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureSettings(path, "/Applications/My Agent/usage-agent"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["theme"] != "dark" || got["enableWorkflows"] != true || got["statusLine"] == nil {
		t.Fatalf("settings not preserved: %s", data)
	}
	backup, err := os.ReadFile(path + ".ai-usage-backup")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(original) {
		t.Fatalf("backup=%s", backup)
	}
}
