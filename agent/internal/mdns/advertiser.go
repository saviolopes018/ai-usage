package mdns

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/pairing"
	"github.com/saviolopes/ai-usage-monitor/agent/internal/protocol"
)

const Service = "_ai-usage._tcp"

type Advertiser struct {
	port      int
	token     string
	mu        sync.Mutex
	server    *mdns.Server
	signature string
}

func Start(port int, token string) (*Advertiser, error) {
	ips, err := localAddresses()
	if err != nil {
		return nil, err
	}
	server, err := registerWithAddresses(port, token, ips)
	if err != nil {
		return nil, err
	}
	return &Advertiser{port: port, token: token, server: server, signature: addressSignature(ips)}, nil
}

// Watch republishes the service whenever the Mac changes network addresses.
func (a *Advertiser) Watch(ctx context.Context, onChange func(), onError func(error)) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed, err := a.refresh()
			if err != nil {
				onError(err)
			} else if changed {
				onChange()
			}
		}
	}
}

func (a *Advertiser) refresh() (bool, error) {
	ips, err := localAddresses()
	if err != nil {
		return false, err
	}
	signature := addressSignature(ips)
	a.mu.Lock()
	defer a.mu.Unlock()
	if signature == a.signature {
		return false, nil
	}
	server, err := registerWithAddresses(a.port, a.token, ips)
	if err != nil {
		return false, err
	}
	old := a.server
	a.server = server
	a.signature = signature
	if old != nil {
		old.Shutdown()
	}
	return true, nil
}

func (a *Advertiser) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server != nil {
		a.server.Shutdown()
		a.server = nil
	}
}

func Register(port int, token string) (*mdns.Server, error) {
	ips, err := localAddresses()
	if err != nil {
		return nil, err
	}
	return registerWithAddresses(port, token, ips)
}

func registerWithAddresses(port int, token string, ips []net.IP) (*mdns.Server, error) {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		name = "AI Usage Monitor"
	}
	name = strings.TrimSuffix(name, ".local")
	txt := []string{
		"id=" + pairing.DeviceID(token),
		"version=1",
		fmt.Sprintf("protocol=%d", protocol.Version),
		"agent=" + protocol.AgentVersion,
		"path=/ws",
	}
	service, err := mdns.NewMDNSService(name, Service, "local.", name+".local.", port, ips, txt)
	if err != nil {
		return nil, fmt.Errorf("create mDNS service: %w", err)
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("register mDNS: %w", err)
	}
	return server, nil
}

func addressSignature(ips []net.IP) string {
	values := make([]string, 0, len(ips))
	for _, ip := range ips {
		values = append(values, ip.String())
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func localAddresses() ([]net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var result []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
				result = append(result, ip)
			}
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("register mDNS: no local addresses")
	}
	return result, nil
}
