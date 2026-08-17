package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/claude"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/codex"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/config"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/launchd"
	usageLogging "github.com/saviolopes/ai-usage-monitor/agent/internal/logging"
	mdnsadvertiser "github.com/saviolopes/ai-usage-monitor/agent/internal/mdns"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/pairing"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/server"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/snapshotcache"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/store"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/tokenusage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return usageError()
	}
	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	switch args[0] {
	case "serve":
		return serve(cfg)
	case "status":
		return status(cfg)
	case "pair":
		if len(args) != 1 {
			return usageError()
		}
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/internal/pairing/create", cfg.Port), nil)
		request.Header.Set("Authorization", "Bearer "+cfg.Token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return fmt.Errorf("agent is not running: %w", err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			return fmt.Errorf("agent refused pairing: HTTP %d", response.StatusCode)
		}
		var created struct{ Ticket, ExpiresAt string }
		if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
			return err
		}
		payload, err := pairing.Build(created.Ticket, pairing.DeviceID(cfg.Token), cfg.Port, created.ExpiresAt)
		if err != nil {
			return err
		}
		qr, err := pairing.TerminalQR(payload)
		if err != nil {
			return err
		}
		fmt.Printf("Pair AI Usage Monitor\nMac: %s\nAddress: %s\nExpires: %s\n\n%s\nThis QR is single-use and expires in 5 minutes.\n", payload.Device, payload.Endpoint, payload.ExpiresAt, qr)
		return nil
	case "devices":
		return deviceCommand(cfg, http.MethodGet, "")
	case "revoke-device":
		if len(args) != 2 {
			return usageError()
		}
		return deviceCommand(cfg, http.MethodDelete, args[1])
	case "claude-statusline":
		data, err := io.ReadAll(io.LimitReader(os.Stdin, claude.MaxPayload+1))
		if err != nil {
			return err
		}
		return claude.StatusLine(data, cfg.Port, os.Stdout)
	case "configure-claude":
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		if err := claude.ConfigureSettings(filepath.Join(home, ".claude", "settings.json"), binary); err != nil {
			return err
		}
		fmt.Println("Claude statusLine configured")
		return nil
	case "install-service":
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		paths, err := launchd.Install(binary)
		if err != nil {
			return err
		}
		if err := claude.ConfigureSettings(filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"), paths.Binary); err != nil {
			return fmt.Errorf("configure Claude statusLine: %w", err)
		}
		fmt.Printf("Agent service installed\nBinary: %s\nLogs: %s\n", paths.Binary, paths.Stdout)
		return nil
	case "service-status":
		state, err := launchd.Status()
		if err != nil {
			return err
		}
		fmt.Println("Agent service:", state)
		return nil
	case "uninstall-service":
		if err := launchd.Uninstall(); err != nil {
			return err
		}
		fmt.Println("Agent service uninstalled; configuration and usage cache preserved")
		return nil
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: usage-agent <serve|status|pair|devices|revoke-device ID|claude-statusline|configure-claude|install-service|service-status|uninstall-service>")
}

func deviceCommand(cfg config.Config, method, id string) error {
	path := "/devices"
	if id != "" {
		path += "/" + id
	}
	req, _ := http.NewRequest(method, fmt.Sprintf("http://localhost:%d%s", cfg.Port, path), nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("agent is not running: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("device command failed: HTTP %d", resp.StatusCode)
	}
	if method == http.MethodDelete {
		fmt.Println("Device revoked")
		return nil
	}
	var devices []config.Device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("No paired devices")
		return nil
	}
	for _, d := range devices {
		fmt.Printf("%s\t%s\t%s\n", d.ID, d.Name, d.CreatedAt)
	}
	return nil
}

func serve(cfg config.Config) error {
	logOutput := io.Writer(os.Stdout)
	var logFile io.Closer
	if path := os.Getenv("AI_USAGE_LOG_FILE"); path != "" {
		writer, err := usageLogging.Open(path, 5*1024*1024, 3)
		if err != nil {
			return fmt.Errorf("open rotating log: %w", err)
		}
		logOutput, logFile = writer, writer
		defer logFile.Close()
	}
	logger := slog.New(slog.NewJSONHandler(logOutput, nil))
	initial := domain.InitialSnapshot()
	snapshotCachePath, snapshotPathErr := snapshotcache.DefaultPath()
	if snapshotPathErr != nil {
		logger.Warn("snapshot.cache.path_failed", "error", snapshotPathErr)
	} else if cached, err := snapshotcache.Load(snapshotCachePath); err == nil {
		initial = snapshotcache.Restore(initial, cached)
		logger.Info("snapshot.cache.restored", "updated_at", cached.UpdatedAt, "stale", snapshotcache.IsStale(cached, time.Now()))
	} else if !os.IsNotExist(err) {
		logger.Warn("snapshot.cache.read_failed", "error", err)
	}
	claudeCachePath, cachePathErr := claude.DefaultCachePath()
	if cachePathErr != nil {
		logger.Warn("claude.cache.path_failed", "error", cachePathErr)
	} else if !initial.Providers[1].Available {
		if cached, err := claude.LoadCache(claudeCachePath); err == nil {
			for i := range initial.Providers {
				if initial.Providers[i].Provider == "claude" {
					initial.Providers[i] = cached
					break
				}
			}
			logger.Info("claude.cache.restored", "observed_at", cached.ObservedAt)
		} else if !os.IsNotExist(err) {
			logger.Warn("claude.cache.read_failed", "error", err)
		}
	}
	st := store.New(initial)
	srv := server.New(cfg.Token, st, logger)
	var configMu sync.Mutex
	srv.ConfigureDevices(cfg.Devices, func(device config.Device) error {
		configMu.Lock()
		defer configMu.Unlock()
		cfg.Devices = append(cfg.Devices, device)
		return config.Save(cfg)
	}, func(device config.Device) error {
		configMu.Lock()
		defer configMu.Unlock()
		for i := range cfg.Devices {
			if cfg.Devices[i].ID == device.ID {
				cfg.Devices[i] = device
				return config.Save(cfg)
			}
		}
		return os.ErrNotExist
	}, func(id string) error {
		configMu.Lock()
		defer configMu.Unlock()
		for i, d := range cfg.Devices {
			if d.ID == id {
				cfg.Devices = append(cfg.Devices[:i], cfg.Devices[i+1:]...)
				return config.Save(cfg)
			}
		}
		return os.ErrNotExist
	})
	runtimeCtx, cancelRuntime := context.WithCancel(context.Background())
	defer cancelRuntime()
	go trackTokens(runtimeCtx, st, logger)
	if snapshotCachePath != "" {
		updates, unsubscribe := st.Subscribe()
		defer unsubscribe()
		go func() {
			for {
				select {
				case snapshot := <-updates:
					if err := snapshotcache.Save(snapshotCachePath, snapshot); err != nil {
						logger.Warn("snapshot.cache.write_failed", "error", err)
					}
				case <-runtimeCtx.Done():
					return
				}
			}
		}()
	}
	mdnsServer, mdnsErr := mdnsadvertiser.Start(cfg.Port, cfg.Token)
	if mdnsErr != nil {
		logger.Warn("mdns.register_failed", "error", mdnsErr)
	} else {
		defer mdnsServer.Shutdown()
		logger.Info("mdns.registered", "service", mdnsadvertiser.Service)
		go mdnsServer.Watch(runtimeCtx,
			func() { logger.Info("mdns.reregistered", "reason", "network addresses changed") },
			func(err error) { logger.Warn("mdns.refresh_failed", "error", err) },
		)
	}
	refresher := claude.Refresher{Binary: "claude"}
	srv.OnClaudeRefresh(func(ctx context.Context) (domain.ProviderUsage, error) {
		return refresher.Refresh(ctx, time.Now())
	})
	if claudeCachePath != "" {
		srv.OnClaudeUpdate(func(usage domain.ProviderUsage) error {
			return claude.SaveCache(claudeCachePath, usage)
		})
	}
	codexCollector := codex.NewCollector(st, logger, "codex")
	srv.OnCodexRefresh(codexCollector.Refresh)
	go codexCollector.Run(runtimeCtx)
	httpServer := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() { logger.Info("agent.started", "port", cfg.Port); errCh <- httpServer.ListenAndServe() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Info("agent.stopping")
		cancelRuntime()
		srv.Close()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func trackTokens(ctx context.Context, st *store.Store, logger *slog.Logger) {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Warn("tokens.home_failed", "error", err)
		return
	}
	refresh := func() {
		usages, scanErr := tokenusage.Scan(home)
		if scanErr != nil {
			logger.Warn("tokens.scan_partial", "error", scanErr)
		}
		for providerName, tokens := range usages {
			snapshot := st.Get()
			for _, provider := range snapshot.Providers {
				if provider.Provider == providerName {
					provider.Tokens = &tokens
					st.UpdateProvider(provider)
					break
				}
			}
		}
	}
	refresh()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			refresh()
		case <-ctx.Done():
			return
		}
	}
}

func status(cfg config.Config) error {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/state", cfg.Port), nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("agent is not reachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent returned %s", resp.Status)
	}
	var snapshot domain.UsageSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return err
	}
	fmt.Println("AI Usage Monitor")
	for _, provider := range snapshot.Providers {
		fmt.Printf("\n%s\n", domain.DisplayName(provider.Provider))
		if !provider.Available {
			fmt.Println("unavailable")
			continue
		}
		printWindow("5h", provider.FiveHour)
		printWindow("Weekly", provider.Weekly)
	}
	return nil
}

func printWindow(label string, window *domain.UsageWindow) {
	if window == nil {
		return
	}
	fmt.Printf("%s\nused: %.0f%%\nremaining: %.0f%%\nreset: %s\n", label, window.UsedPercentage, window.RemainingPercentage, window.ResetsAt.Format(time.RFC3339))
}
