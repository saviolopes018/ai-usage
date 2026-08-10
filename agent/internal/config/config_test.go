package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadOrCreateAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	first, err := LoadOrCreateAt(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Token == "" || first.Token != second.Token || first.Port != 9876 {
		t.Fatalf("unexpected configs: %+v %+v", first, second)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestConcurrentCreationUsesOneToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	const workers = 20
	configs := make(chan Config, workers)
	errors := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg, err := LoadOrCreateAt(path)
			if err != nil {
				errors <- err
				return
			}
			configs <- cfg
		}()
	}
	wg.Wait()
	close(configs)
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	var token string
	for cfg := range configs {
		if token == "" {
			token = cfg.Token
		}
		if cfg.Token != token {
			t.Fatalf("concurrent callers received different tokens")
		}
	}
}

func TestRejectsSymlinkConfig(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(`{"token":"secret","port":9876}`), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateAt(path); err == nil {
		t.Fatal("expected symlink config to be rejected")
	}
}

func TestLoadOrCreateRepairsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"token":"secret","port":9876}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateAt(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
