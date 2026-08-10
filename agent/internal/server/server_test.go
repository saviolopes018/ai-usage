package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/config"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/domain"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/store"
)

func testServer(t *testing.T) (*Server, *store.Store, *httptest.Server) {
	t.Helper()
	st := store.New(domain.InitialSnapshot())
	srv := New("secret", st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { srv.Close(); ts.Close() })
	return srv, st, ts
}

func TestUnsupportedProtocolIsRejected(t *testing.T) {
	_, _, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/ws?protocol=99&token=secret")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestPairingTicketIsSingleUseAndCredentialCanBeRevoked(t *testing.T) {
	srv, _, ts := testServer(t)
	devices := map[string]config.Device{}
	srv.ConfigureDevices(nil, func(device config.Device) error { devices[device.ID] = device; return nil }, func(device config.Device) error { devices[device.ID] = device; return nil }, func(id string) error {
		if _, ok := devices[id]; !ok {
			return errors.New("missing")
		}
		delete(devices, id)
		return nil
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/internal/pairing/create", nil)
	req.Header.Set("Authorization", "Bearer secret")
	created, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer created.Body.Close()
	var ticket map[string]string
	if err := json.NewDecoder(created.Body).Decode(&ticket); err != nil {
		t.Fatal(err)
	}
	claimBody := func() *bytes.Reader {
		return bytes.NewReader([]byte(`{"ticket":"` + ticket["ticket"] + `","name":"Savio iPhone"}`))
	}
	claimed, err := http.Post(ts.URL+"/pair/claim", "application/json", claimBody())
	if err != nil {
		t.Fatal(err)
	}
	var credential map[string]string
	if err := json.NewDecoder(claimed.Body).Decode(&credential); err != nil {
		t.Fatal(err)
	}
	claimed.Body.Close()
	if credential["token"] == "" || credential["credentialId"] == "" {
		t.Fatalf("credential=%v", credential)
	}
	second, err := http.Post(ts.URL+"/pair/claim", "application/json", claimBody())
	if err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if second.StatusCode != http.StatusGone {
		t.Fatalf("second status=%d", second.StatusCode)
	}
	state, err := http.Get(ts.URL + "/state?token=" + credential["token"])
	if err != nil {
		t.Fatal(err)
	}
	state.Body.Close()
	if state.StatusCode != http.StatusOK {
		t.Fatalf("device auth status=%d", state.StatusCode)
	}
	revoke, _ := http.NewRequest(http.MethodDelete, ts.URL+"/devices/"+credential["credentialId"], nil)
	revoke.Header.Set("Authorization", "Bearer secret")
	revoked, err := http.DefaultClient.Do(revoke)
	if err != nil {
		t.Fatal(err)
	}
	revoked.Body.Close()
	if revoked.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke status=%d", revoked.StatusCode)
	}
	denied, err := http.Get(ts.URL + "/state?token=" + credential["token"])
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked auth status=%d", denied.StatusCode)
	}
}

func TestAuthentication(t *testing.T) {
	_, _, ts := testServer(t)
	for _, path := range []string{"/state", "/ws"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s status=%d", path, resp.StatusCode)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	raw, _ := http.NewRequest(http.MethodGet, ts.URL+"/state", nil)
	raw.Header.Set("Authorization", "secret")
	rawResp, err := http.DefaultClient.Do(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer rawResp.Body.Close()
	if rawResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("raw token status=%d", rawResp.StatusCode)
	}
}

func TestMasterTokenIsRejectedFromLAN(t *testing.T) {
	srv, _, _ := testServer(t)
	srv.ConfigureDevices([]config.Device{{ID: "phone", Name: "iPhone", Token: "device-token"}}, func(config.Device) error { return nil }, func(config.Device) error { return nil }, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "http://192.168.1.2/state", nil)
	req.RemoteAddr = "192.168.1.20:54321"
	req.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestWebSocketAuthenticatesWithSubprotocolWithoutTokenInURL(t *testing.T) {
	_, _, ts := testServer(t)
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?protocol=1"
	dialer := websocket.Dialer{Subprotocols: []string{"ai-usage.v1", "auth.secret"}}
	conn, response, err := dialer.Dial(url, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("status=%d err=%v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	defer conn.Close()
	if conn.Subprotocol() != "ai-usage.v1" {
		t.Fatalf("subprotocol=%q", conn.Subprotocol())
	}
	var snapshot domain.UsageSnapshot
	if err := conn.ReadJSON(&snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketMultipleClients(t *testing.T) {
	_, st, ts := testServer(t)
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?token=secret"
	connections := make([]*websocket.Conn, 3)
	for i := range connections {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		connections[i] = conn
		var initial domain.UsageSnapshot
		if err := conn.ReadJSON(&initial); err != nil {
			t.Fatal(err)
		}
	}
	st.UpdateProvider(domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: time.Now().UTC()})
	for _, conn := range connections {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var updated domain.UsageSnapshot
		if err := conn.ReadJSON(&updated); err != nil {
			t.Fatal(err)
		}
		if !updated.Providers[0].Available {
			t.Fatalf("client received stale state: %+v", updated)
		}
	}
}

func TestHealthIsPublic(t *testing.T) {
	_, _, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestWebSocketInitialAndUpdatedSnapshot(t *testing.T) {
	_, st, ts := testServer(t)
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?token=secret"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var initial domain.UsageSnapshot
	if err := conn.ReadJSON(&initial); err != nil {
		t.Fatal(err)
	}
	if len(initial.Providers) != 2 {
		t.Fatalf("unexpected initial: %+v", initial)
	}
	st.UpdateProvider(domain.ProviderUsage{Provider: "codex", Available: true, ObservedAt: time.Now().UTC()})
	var updated domain.UsageSnapshot
	if err := conn.ReadJSON(&updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Providers[0].Available {
		t.Fatalf("unexpected update: %+v", updated)
	}
}

func TestStateJSON(t *testing.T) {
	_, _, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/state?token=secret")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got domain.UsageSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Device == "" {
		t.Fatal("device missing")
	}
}

func TestClaudeStatusEndpoint(t *testing.T) {
	_, st, ts := testServer(t)
	resp, err := http.Post(ts.URL+"/internal/claude/status", "application/json", bytes.NewBufferString(`{"rate_limits":{"five_hour":{"used_percentage":30}}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if !st.Get().Providers[1].Available || st.Get().Providers[1].FiveHour.UsedPercentage != 30 {
		t.Fatalf("state=%+v", st.Get())
	}
}

func TestClaudeRefreshIsAuthenticatedAndUpdatesState(t *testing.T) {
	srv, st, ts := testServer(t)
	srv.OnClaudeRefresh(func(context.Context) (domain.ProviderUsage, error) {
		return domain.ProviderUsage{
			Provider: "claude", Available: true, ObservedAt: time.Now().UTC(),
			Weekly: &domain.UsageWindow{UsedPercentage: 24, RemainingPercentage: 76},
		}, nil
	})
	unauthorized, err := http.Post(ts.URL+"/claude/refresh", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.StatusCode)
	}
	response, err := http.Post(ts.URL+"/claude/refresh?token=secret", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", response.StatusCode)
	}
	if got := st.Get().Providers[1]; !got.Available || got.Weekly == nil || got.Weekly.UsedPercentage != 24 {
		t.Fatalf("state=%+v", got)
	}
}

func TestCodexRefreshIsAuthenticatedAndTriggersCollector(t *testing.T) {
	srv, _, ts := testServer(t)
	called := make(chan struct{}, 1)
	srv.OnCodexRefresh(func(context.Context) error {
		called <- struct{}{}
		return nil
	})
	unauthorized, err := http.Post(ts.URL+"/codex/refresh", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.StatusCode)
	}
	response, err := http.Post(ts.URL+"/codex/refresh?token=secret", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("status=%d", response.StatusCode)
	}
	select {
	case <-called:
	default:
		t.Fatal("collector was not triggered")
	}
}
func TestClaudeStatusRejectsInvalidPayload(t *testing.T) {
	_, _, ts := testServer(t)
	resp, err := http.Post(ts.URL+"/internal/claude/status", "application/json", strings.NewReader(`{"rate_limits":null}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestClaudeStatusRejectsProxiedRequest(t *testing.T) {
	_, _, ts := testServer(t)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/internal/claude/status", strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":30}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
