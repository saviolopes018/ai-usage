package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/saviolopes/ai-usage-monitor/agent/internal/config"
)

type pairingTicket struct{ expires time.Time }

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Server) createPairing(w http.ResponseWriter, _ *http.Request) {
	token, err := randomToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create pairing"})
		return
	}
	expires := time.Now().Add(5 * time.Minute)
	s.authMu.Lock()
	for existing, pending := range s.tickets {
		if time.Now().After(pending.expires) {
			delete(s.tickets, existing)
		}
	}
	s.tickets[token] = pairingTicket{expires: expires}
	s.authMu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]string{"ticket": token, "expiresAt": expires.UTC().Format(time.RFC3339)})
}

func (s *Server) claimPairing(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Ticket string `json:"ticket"`
		Name   string `json:"name"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := decodeJSON(r, &input); err != nil || len(input.Ticket) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "iPhone"
	}
	if len(input.Name) > 80 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device name"})
		return
	}
	s.authMu.Lock()
	ticket, ok := s.tickets[input.Ticket]
	delete(s.tickets, input.Ticket)
	s.authMu.Unlock()
	if !ok || time.Now().After(ticket.expires) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "pairing expired or already used"})
		return
	}
	token, err := randomToken()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "could not create credential"})
		return
	}
	idRaw, err := randomToken()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "could not create credential"})
		return
	}
	device := config.Device{ID: idRaw[:16], Name: input.Name, Token: token, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	if s.saveDevice == nil || s.saveDevice(device) != nil {
		writeJSON(w, 500, map[string]string{"error": "could not save credential"})
		return
	}
	s.authMu.Lock()
	s.deviceTokens[token] = device
	s.authMu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]string{"token": token, "credentialId": device.ID})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (s *Server) listDevices(w http.ResponseWriter, _ *http.Request) {
	s.authMu.RLock()
	devices := make([]config.Device, 0, len(s.deviceTokens))
	for _, d := range s.deviceTokens {
		d.Token = ""
		devices = append(devices, d)
	}
	s.authMu.RUnlock()
	writeJSON(w, http.StatusOK, devices)
}

func (s *Server) revokeDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "missing id"})
		return
	}
	if s.revokeDeviceFn == nil || s.revokeDeviceFn(id) != nil {
		writeJSON(w, 404, map[string]string{"error": "device not found"})
		return
	}
	s.authMu.Lock()
	for token, d := range s.deviceTokens {
		if d.ID == id {
			delete(s.deviceTokens, token)
		}
	}
	s.authMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
