package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type Config struct {
	Token   string   `json:"token"`
	Port    int      `json:"port"`
	Devices []Device `json:"devices,omitempty"`
}

type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	CreatedAt string `json:"createdAt"`
	LastSeen  string `json:"lastSeen,omitempty"`
}

func LoadOrCreate() (Config, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	return LoadOrCreateAt(filepath.Join(dir, ".ai-usage", "config.json"))
}

func Save(cfg Config) error {
	dir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return SaveAt(filepath.Join(dir, ".ai-usage", "config.json"), cfg)
}

func SaveAt(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadOrCreateAt(path string) (Config, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Config{}, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return Config{}, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return Config{}, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck -- best effort during close
	return loadOrCreateLocked(path)
}

func loadOrCreateLocked(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		var cfg Config
		if err := json.Unmarshal(b, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode config: %w", err)
		}
		if cfg.Token == "" || cfg.Port < 1 || cfg.Port > 65535 {
			return Config{}, fmt.Errorf("invalid config at %s", path)
		}
		if err := securePermissions(path); err != nil {
			return Config{}, fmt.Errorf("secure config permissions: %w", err)
		}
		return cfg, nil
	}
	if !os.IsNotExist(err) {
		return Config{}, err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return Config{}, err
	}
	cfg := Config{Token: base64.RawURLEncoding.EncodeToString(random), Port: 9876}
	b, _ = json.MarshalIndent(cfg, "", "  ")
	b = append(b, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return loadOrCreateLocked(path)
	}
	if err != nil {
		return Config{}, err
	}
	if _, err := file.Write(b); err != nil {
		_ = file.Close()
		return Config{}, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return Config{}, err
	}
	if err := file.Close(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func securePermissions(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("config is not a regular file")
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}
