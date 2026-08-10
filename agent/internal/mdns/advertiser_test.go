package mdns

import (
	"net"
	"testing"
)

func TestAddressSignatureIsStableAcrossInterfaceOrder(t *testing.T) {
	first := []net.IP{net.ParseIP("192.168.1.20"), net.ParseIP("fe80::1")}
	second := []net.IP{net.ParseIP("fe80::1"), net.ParseIP("192.168.1.20")}
	if addressSignature(first) != addressSignature(second) {
		t.Fatal("address signature depends on interface order")
	}
	if addressSignature(first) == addressSignature([]net.IP{net.ParseIP("192.168.1.21"), net.ParseIP("fe80::1")}) {
		t.Fatal("address change was not detected")
	}
}
