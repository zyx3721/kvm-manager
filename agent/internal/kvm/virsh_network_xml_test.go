package kvm

import (
	"strings"
	"testing"
)

func TestFixedAddressEntriesFollowSubnetRange(t *testing.T) {
	items, err := fixedAddressEntries("192.168.100.0/24")
	if err != nil {
		t.Fatalf("expected fixed addresses, got %v", err)
	}
	if len(items) != 253 {
		t.Fatalf("expected 253 fixed addresses, got %d", len(items))
	}
	if items[0].Address != "192.168.100.2" {
		t.Fatalf("expected first fixed address 192.168.100.2, got %s", items[0].Address)
	}
	if items[len(items)-1].Address != "192.168.100.254" {
		t.Fatalf("expected last fixed address 192.168.100.254, got %s", items[len(items)-1].Address)
	}
	if items[0].MAC == items[1].MAC {
		t.Fatal("expected generated MAC addresses to be unique")
	}
}

func TestFixedAddressEntriesRespectSmallSubnet(t *testing.T) {
	items, err := fixedAddressEntries("10.0.0.8/29")
	if err != nil {
		t.Fatalf("expected fixed addresses, got %v", err)
	}
	addresses := make([]string, 0, len(items))
	for _, item := range items {
		addresses = append(addresses, item.Address)
	}
	expected := []string{"10.0.0.10", "10.0.0.11", "10.0.0.12", "10.0.0.13", "10.0.0.14"}
	if strings.Join(addresses, ",") != strings.Join(expected, ",") {
		t.Fatalf("expected %v, got %v", expected, addresses)
	}
}

func TestNetworkXMLRejectsTooSmallDHCPSubnet(t *testing.T) {
	_, err := networkXML(NetworkPoolCreateRequest{
		Name:   "small",
		Type:   "nat",
		Subnet: "192.168.1.0/31",
		DHCP:   true,
	})
	if err == nil {
		t.Fatal("expected subnet too small validation error")
	}
}

func TestNetworkXMLRejectsTooLargeFixedAddressRange(t *testing.T) {
	_, err := networkXML(NetworkPoolCreateRequest{
		Name:         "large",
		Type:         "nat",
		Subnet:       "10.0.0.0/16",
		DHCP:         true,
		FixedAddress: true,
	})
	if err == nil {
		t.Fatal("expected fixed address range too large validation error")
	}
}

func TestCIDRNetworkUsesActualPrefix(t *testing.T) {
	gateway, netmask, start, end := cidrNetwork("192.168.100.128/25")
	if gateway != "192.168.100.129" || netmask != "255.255.255.128" || start != "192.168.100.130" || end != "192.168.100.254" {
		t.Fatalf("unexpected cidr network values: gateway=%s netmask=%s start=%s end=%s", gateway, netmask, start, end)
	}
}

func TestNetworkXMLWritesFixedAddressesForSubnet(t *testing.T) {
	content, err := networkXML(NetworkPoolCreateRequest{
		Name:         "demo",
		Type:         "nat",
		Subnet:       "192.168.100.0/24",
		DHCP:         true,
		FixedAddress: true,
	})
	if err != nil {
		t.Fatalf("expected network xml, got %v", err)
	}
	if count := strings.Count(content, "<host "); count != 253 {
		t.Fatalf("expected 253 fixed address hosts, got %d", count)
	}
	if !strings.Contains(content, "ip='192.168.100.2'") || !strings.Contains(content, "ip='192.168.100.254'") {
		t.Fatal("expected fixed address hosts to cover subnet usable range after gateway")
	}
}
