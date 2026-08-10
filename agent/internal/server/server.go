package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/claude"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/config"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/protocol"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/store"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
)

type Server struct {
	token           string
	store           *store.Store
	logger          *slog.Logger
	hub             *Hub
	claudeUpdate    func(domain.ProviderUsage) error
	claudeRefresh   func(context.Context) (domain.ProviderUsage, error)
	claudeRefreshMu sync.Mutex
	codexRefresh    func(context.Context) error
	authMu          sync.RWMutex
	deviceTokens    map[string]config.Device
	tickets         map[string]pairingTicket
	saveDevice      func(config.Device) error
	updateDevice    func(config.Device) error
	revokeDeviceFn  func(string) error
}

func New(token string, st *store.Store, logger *slog.Logger) *Server {
	return &Server{token: token, store: st, logger: logger, hub: NewHub(logger), deviceTokens: map[string]config.Device{}, tickets: map[string]pairingTicket{}}
}
func (s *Server) ConfigureDevices(devices []config.Device, save func(config.Device) error, update func(config.Device) error, revoke func(string) error) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	for _, device := range devices {
		s.deviceTokens[device.Token] = device
	}
	s.saveDevice, s.updateDevice, s.revokeDeviceFn = save, update, revoke
}
func (s *Server) OnClaudeUpdate(fn func(domain.ProviderUsage) error) { s.claudeUpdate = fn }
func (s *Server) OnClaudeRefresh(fn func(context.Context) (domain.ProviderUsage, error)) {
	s.claudeRefresh = fn
}
func (s *Server) OnCodexRefresh(fn func(context.Context) error) { s.codexRefresh = fn }
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /state", s.auth(s.state))
	mux.HandleFunc("GET /ws", s.ws)
	mux.HandleFunc("POST /internal/claude/status", s.claudeStatus)
	mux.HandleFunc("POST /claude/refresh", s.auth(s.refreshClaude))
	mux.HandleFunc("POST /codex/refresh", s.auth(s.refreshCodex))
	mux.HandleFunc("GET /auth/info", s.auth(s.authInfo))
	mux.HandleFunc("POST /internal/pairing/create", s.masterAuth(s.createPairing))
	mux.HandleFunc("POST /pair/claim", s.claimPairing)
	mux.HandleFunc("GET /devices", s.auth(s.listDevices))
	mux.HandleFunc("DELETE /devices/{id}", s.auth(s.revokeDevice))
	return mux
}
func (s *Server) refreshCodex(w http.ResponseWriter, r *http.Request) {
	if s.codexRefresh == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Codex refresh unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.codexRefresh(ctx); err != nil {
		s.logger.Warn("codex.refresh.failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Could not refresh Codex usage"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "refreshing"})
}
func (s *Server) refreshClaude(w http.ResponseWriter, r *http.Request) {
	if s.claudeRefresh == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Claude refresh unavailable"})
		return
	}
	if !s.claudeRefreshMu.TryLock() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Claude refresh already running"})
		return
	}
	defer s.claudeRefreshMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	usage, err := s.claudeRefresh(ctx)
	if err != nil {
		s.logger.Warn("claude.refresh.failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Could not refresh Claude usage"})
		return
	}
	s.store.UpdateProvider(usage)
	if s.claudeUpdate != nil {
		if err := s.claudeUpdate(usage); err != nil {
			s.logger.Warn("claude.cache.write_failed", "error", err)
		}
	}
	s.logger.Info("claude.refresh.updated")
	writeJSON(w, http.StatusOK, usage)
}
func (s *Server) claudeStatus(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	requestHost := r.Host
	if parsed, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
		requestHost = parsed
	}
	requestIP := net.ParseIP(requestHost)
	localHost := requestHost == "localhost" || (requestIP != nil && requestIP.IsLoopback())
	proxied := r.Header.Get("Forwarded") != "" || r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Forwarded-Host") != ""
	if err != nil || !net.ParseIP(host).IsLoopback() || !localHost || proxied {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "localhost only"})
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "application/json required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, claude.MaxPayload)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "invalid payload"})
		return
	}
	usage, err := claude.Parse(data, time.Now())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	s.store.UpdateProvider(usage)
	if s.claudeUpdate != nil {
		if err := s.claudeUpdate(usage); err != nil {
			s.logger.Warn("claude.cache.write_failed", "error", err)
		}
	}
	s.logger.Info("claude.rate_limits.updated")
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) Close() { s.hub.Close() }
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "online": true})
}
func (s *Server) state(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Get())
}
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isValidToken(requestToken(r), requestIsLoopback(r)) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}
func (s *Server) masterAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requestIsLoopback(r) || !validToken(s.token, requestToken(r)) {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}
func (s *Server) isValidToken(got string, allowMaster bool) bool {
	if validToken(s.token, got) {
		s.authMu.RLock()
		hasDevices := len(s.deviceTokens) > 0
		s.authMu.RUnlock()
		if allowMaster || !hasDevices {
			return true
		}
		return false
	}
	s.authMu.Lock()
	device, ok := s.deviceTokens[got]
	shouldPersist := false
	if ok {
		lastSeen, _ := time.Parse(time.RFC3339, device.LastSeen)
		shouldPersist = time.Since(lastSeen) >= time.Minute
		device.LastSeen = time.Now().UTC().Format(time.RFC3339)
		s.deviceTokens[got] = device
	}
	s.authMu.Unlock()
	if shouldPersist && s.updateDevice != nil {
		_ = s.updateDevice(device)
	}
	return ok
}
func requestIsLoopback(r *http.Request) bool {
	if r.Header.Get("Forwarded") != "" || r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
func requestToken(r *http.Request) string {
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") {
		for _, candidate := range websocket.Subprotocols(r) {
			if strings.HasPrefix(candidate, "auth.") {
				return strings.TrimPrefix(candidate, "auth.")
			}
		}
		return ""
	}
	return strings.TrimPrefix(authorization, "Bearer ")
}
func (s *Server) authInfo(w http.ResponseWriter, r *http.Request) {
	token := requestToken(r)
	if validToken(s.token, token) {
		writeJSON(w, http.StatusOK, map[string]any{"kind": "master", "migrationRequired": true})
		return
	}
	s.authMu.RLock()
	device, ok := s.deviceTokens[token]
	s.authMu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": "device", "migrationRequired": false, "credentialId": device.ID})
}
func validToken(want, got string) bool {
	return len(want) == len(got) && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	if requested := r.URL.Query().Get("protocol"); requested != "" && requested != "1" {
		writeJSON(w, http.StatusUpgradeRequired, map[string]any{"error": "unsupported protocol", "supportedProtocol": protocol.Version})
		return
	}
	if !s.isValidToken(requestToken(r), requestIsLoopback(r)) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	upgrader := websocket.Upgrader{Subprotocols: []string{"ai-usage.v1"}, CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("websocket.upgrade_failed", "error", err)
		return
	}
	client, err := s.hub.Add(conn)
	if err != nil {
		_ = conn.Close()
		return
	}
	updates, unsubscribe := s.store.SubscribeWithInitial()
	go func() {
		defer unsubscribe()
		for {
			select {
			case snapshot := <-updates:
				if err := client.Send(snapshot); err != nil {
					return
				}
			case <-client.Done():
				return
			}
		}
	}()
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

type Client struct {
	conn *websocket.Conn
	send chan domain.UsageSnapshot
	done chan struct{}
	hub  *Hub
}
type Hub struct {
	logger *slog.Logger
	add    chan *Client
	remove chan *Client
	close  chan struct{}
}

func NewHub(logger *slog.Logger) *Hub {
	h := &Hub{logger: logger, add: make(chan *Client), remove: make(chan *Client), close: make(chan struct{})}
	go h.run()
	return h
}
func (h *Hub) run() {
	clients := map[*Client]struct{}{}
	for {
		select {
		case c := <-h.add:
			clients[c] = struct{}{}
			h.logger.Info("websocket.connected", "clients", len(clients))
		case c := <-h.remove:
			if _, ok := clients[c]; ok {
				delete(clients, c)
				close(c.done)
				_ = c.conn.Close()
				h.logger.Info("websocket.disconnected", "clients", len(clients))
			}
		case <-h.close:
			for c := range clients {
				close(c.done)
				_ = c.conn.Close()
			}
			return
		}
	}
}
func (h *Hub) Add(conn *websocket.Conn) (*Client, error) {
	c := &Client{conn: conn, send: make(chan domain.UsageSnapshot, 8), done: make(chan struct{}), hub: h}
	select {
	case h.add <- c:
	case <-h.close:
		return nil, errors.New("websocket hub is closed")
	}
	go c.writeLoop()
	go c.readLoop()
	return c, nil
}
func (h *Hub) Remove(c *Client) {
	select {
	case h.remove <- c:
	case <-h.close:
	}
}
func (h *Hub) Close() {
	select {
	case <-h.close:
	default:
		close(h.close)
	}
}
func (c *Client) Send(snapshot domain.UsageSnapshot) error {
	select {
	case c.send <- snapshot:
		return nil
	case <-c.done:
		return websocket.ErrCloseSent
	}
}
func (c *Client) Done() <-chan struct{} { return c.done }
func (c *Client) readLoop() {
	defer c.hub.Remove(c)
	c.conn.SetReadLimit(1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case snapshot := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteJSON(snapshot); err != nil {
				c.hub.Remove(c)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.hub.Remove(c)
				return
			}
		case <-c.done:
			return
		}
	}
}
