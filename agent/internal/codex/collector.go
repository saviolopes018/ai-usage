package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
)

type StateStore interface {
	Get() domain.UsageSnapshot
	UpdateProvider(domain.ProviderUsage) bool
}

type ProcessManager struct{ Binary string }

func (p ProcessManager) Command(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, p.Binary, "app-server", "--stdio")
}

type Collector struct {
	store   StateStore
	logger  *slog.Logger
	process ProcessManager
	refresh chan chan error
}

func NewCollector(store StateStore, logger *slog.Logger, binary string) *Collector {
	return &Collector{store: store, logger: logger, process: ProcessManager{Binary: binary}, refresh: make(chan chan error)}
}

// Refresh requests a fresh reading from the long-lived app-server process.
func (c *Collector) Refresh(ctx context.Context) error {
	result := make(chan error, 1)
	select {
	case c.refresh <- result:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Collector) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := c.runOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		c.markUnavailable()
		c.logger.Warn("codex.disconnected", "error", err)
		c.logger.Info("codex.reconnecting", "backoff", backoff.String())
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		if backoff < 10*time.Second {
			backoff *= 2
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
		}
	}
}

func (c *Collector) runOnce(ctx context.Context) error {
	processCtx, cancelProcess := context.WithCancel(ctx)
	defer cancelProcess()
	cmd := c.process.Command(processCtx)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		cancelProcess()
		_ = cmd.Wait()
	}()
	c.logger.Info("codex.app_server.started", "pid", cmd.Process.Pid)
	go drain(stderr)
	client := &JsonRPCClient{in: bufio.NewScanner(stdout), out: stdin, nextID: 2}
	client.in.Buffer(make([]byte, 64*1024), 1024*1024)
	if err := client.write(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]string{"name": "ai-usage-monitor", "version": "0.1.0"}, "capabilities": map[string]any{}}}); err != nil {
		return err
	}
	if err := client.awaitResponse(1); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if err := client.write(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
		return err
	}
	if err := client.requestRateLimits(); err != nil {
		return err
	}
	c.logger.Info("codex.connected")
	messages := make(chan rpcMessage)
	scanErrors := make(chan error, 1)
	go func() {
		for {
			message, err := client.scan()
			if err != nil {
				scanErrors <- err
				return
			}
			select {
			case messages <- message:
			case <-processCtx.Done():
				return
			}
		}
	}()
	poll := time.NewTicker(time.Minute)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = stdin.Close()
			return ctx.Err()
		case err := <-scanErrors:
			return err
		case <-poll.C:
			if err := client.requestRateLimits(); err != nil {
				return err
			}
		case result := <-c.refresh:
			err := client.requestRateLimits()
			result <- err
			if err != nil {
				return err
			}
		case message := <-messages:
			if message.ID != nil && message.Result != nil {
				usage, err := MapRateLimits(message.Result, time.Now())
				if err != nil {
					c.logger.Warn("codex.rate_limits.invalid", "error", err)
					continue
				}
				c.store.UpdateProvider(usage)
				c.logger.Info("codex.rate_limits.updated")
			} else if message.Method == "account/rateLimits/updated" {
				if err := client.requestRateLimits(); err != nil {
					return err
				}
			}
		}
	}
}

func (c *Collector) markUnavailable() {
	for _, p := range c.store.Get().Providers {
		if p.Provider == "codex" {
			p.Available = false
			c.store.UpdateProvider(p)
			return
		}
	}
}

type rpcMessage struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}
type JsonRPCClient struct {
	in     *bufio.Scanner
	out    io.Writer
	mu     sync.Mutex
	nextID int
}

func (c *JsonRPCClient) write(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return json.NewEncoder(c.out).Encode(v)
}
func (c *JsonRPCClient) scan() (rpcMessage, error) {
	if !c.in.Scan() {
		if err := c.in.Err(); err != nil {
			return rpcMessage{}, err
		}
		return rpcMessage{}, io.EOF
	}
	var m rpcMessage
	if err := json.Unmarshal(c.in.Bytes(), &m); err != nil {
		return rpcMessage{}, err
	}
	return m, nil
}
func (c *JsonRPCClient) awaitResponse(id int) error {
	for {
		m, err := c.scan()
		if err != nil {
			return err
		}
		if m.ID != nil && *m.ID == id {
			if m.Error != nil {
				return errors.New(m.Error.Message)
			}
			return nil
		}
	}
}
func (c *JsonRPCClient) requestRateLimits() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextID
	c.nextID++
	return json.NewEncoder(c.out).Encode(map[string]any{"id": id, "method": "account/rateLimits/read", "params": map[string]any{}})
}
func drain(r io.Reader) { _, _ = io.Copy(io.Discard, r) }
