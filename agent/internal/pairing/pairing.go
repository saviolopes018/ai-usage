package pairing

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const Type = "ai-usage-monitor-pairing"
const Version = 2

type Payload struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	Endpoint  string `json:"endpoint"`
	Ticket    string `json:"ticket"`
	Device    string `json:"device"`
	DeviceID  string `json:"deviceId"`
	ExpiresAt string `json:"expiresAt"`
}

func DeviceID(token string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))[:16] }

func Build(ticket string, deviceID string, port int, expiresAt string) (Payload, error) {
	if ticket == "" || deviceID == "" || port < 1 || port > 65535 {
		return Payload{}, errors.New("invalid pairing configuration")
	}
	ip, err := localIPv4()
	if err != nil {
		return Payload{}, err
	}
	device, err := os.Hostname()
	if err != nil || strings.TrimSpace(device) == "" {
		device = "Mac"
	}
	return Payload{Type: Type, Version: Version, Endpoint: net.JoinHostPort(ip.String(), fmt.Sprint(port)), Ticket: ticket, Device: device, DeviceID: deviceID, ExpiresAt: expiresAt}, nil
}

func Encode(payload Payload) (string, error) {
	data, err := json.Marshal(payload)
	return string(data), err
}

func TerminalQR(payload Payload) (string, error) {
	data, err := Encode(payload)
	if err != nil {
		return "", err
	}
	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return "", err
	}
	return qr.ToSmallString(false), nil
}

func localIPv4() (net.IP, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if address, ok := conn.LocalAddr().(*net.UDPAddr); ok && address.IP.IsPrivate() {
			return address.IP, nil
		}
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil && ip.IsPrivate() {
				return ip, nil
			}
		}
	}
	return nil, errors.New("no private IPv4 address found; connect the Mac to Wi-Fi")
}
