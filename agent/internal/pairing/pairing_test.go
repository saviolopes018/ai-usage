package pairing

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeAndTerminalQR(t *testing.T) {
	payload := Payload{Type: Type, Version: Version, Endpoint: "192.168.1.8:9876", Ticket: "temporary-secret", Device: "Mac", DeviceID: DeviceID("secret"), ExpiresAt: "2099-01-01T00:00:00Z"}
	encoded, err := Encode(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Payload
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Endpoint != payload.Endpoint || decoded.Ticket != payload.Ticket {
		t.Fatalf("decoded=%+v", decoded)
	}
	qr, err := TerminalQR(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(qr, "▀") && !strings.Contains(qr, "▄") {
		t.Fatal("terminal QR is empty")
	}
}

func TestDeviceIDIsStableAndDoesNotExposeToken(t *testing.T) {
	first := DeviceID("secret")
	if first != DeviceID("secret") || first == "secret" || len(first) != 16 {
		t.Fatalf("id=%q", first)
	}
}
