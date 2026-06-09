package kvm

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseInterfaceListNames(t *testing.T) {
	output := ` Name           State    MAC Address
---------------------------------------------------
 br0            active   34:48:ed:f3:41:7c
 brvlan225      active   34:48:ed:f3:41:7d
 em3            inactive 34:48:ed:f3:41:7e
`

	result := parseInterfaceListNames(output)

	expected := []string{"br0", "brvlan225", "em3"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d names, got %d", len(expected), len(result))
	}
	for index, name := range expected {
		if result[index] != name {
			t.Fatalf("expected name %s at index %d, got %s", name, index, result[index])
		}
	}
}

func TestAppendUniqueInterfaceListEntriesSkipsDuplicates(t *testing.T) {
	activeOutput := ` Name           State    MAC Address
---------------------------------------------------
 br0            active   34:48:ed:f3:41:7c
 em1            active   34:48:ed:f3:41:7d
`
	inactiveOutput := ` Name           State    MAC Address
---------------------------------------------------
 br0            inactive 34:48:ed:f3:41:7c
 br1            inactive 34:48:ed:f3:41:7e
`

	seen := map[string]struct{}{}
	entries := appendUniqueInterfaceListEntries(nil, seen, activeOutput)
	entries = appendUniqueInterfaceListEntries(entries, seen, inactiveOutput)

	if len(entries) != 3 {
		t.Fatalf("expected 3 unique entries, got %d", len(entries))
	}
	if entries[0].Name != "br0" || entries[0].Status != "up" {
		t.Fatalf("expected first br0 active entry to be kept, got %+v", entries[0])
	}
	if entries[2].Name != "br1" || entries[2].Status != "down" {
		t.Fatalf("expected inactive br1 entry to be appended, got %+v", entries[2])
	}
}

func TestInterfaceXMLDocParsesBridgeDevice(t *testing.T) {
	xmlText := `<interface type='bridge' name='br0'>
  <bridge>
    <interface type='ethernet' name='eth0'>
      <link speed='10000' state='up'/>
      <mac address='00:50:56:ab:ae:96'/>
    </interface>
  </bridge>
</interface>`
	var doc interfaceXMLDoc
	if err := xml.Unmarshal([]byte(xmlText), &doc); err != nil {
		t.Fatalf("expected bridge XML to parse, got %v", err)
	}
	if doc.Bridge.Interface.Name != "eth0" {
		t.Fatalf("expected bridge device eth0, got %q", doc.Bridge.Interface.Name)
	}
}

func TestMergeHostInterfacesKeepsLibvirtListNarrow(t *testing.T) {
	primary := []HostInterface{
		{Name: "lo", Type: "loopback"},
		{Name: "br0", Type: "bridge"},
	}
	secondary := []HostInterface{
		{Name: "br0", MAC: "52:54:00:00:00:01", Status: "up", IPv4: "192.168.1.10/24", IPv4Mode: "static"},
		{Name: "vnet4", MAC: "fe:54:00:00:00:04", Status: "up"},
		{Name: "em2.225", MAC: "52:54:00:00:02:25", Status: "up"},
	}

	result := mergeHostInterfaces(primary, secondary, false)

	if len(result) != 2 {
		t.Fatalf("expected only libvirt interfaces, got %d", len(result))
	}
	if result[1].Name != "br0" || result[1].MAC == "" || result[1].IPv4 == "" {
		t.Fatalf("expected same-name runtime fields to supplement br0, got %+v", result[1])
	}
	for _, item := range result {
		if item.Name == "vnet4" || item.Name == "em2.225" {
			t.Fatalf("expected runtime-only interface %s to be hidden", item.Name)
		}
	}
}

func TestMergeHostInterfacesPromotesRuntimeIPv6LinkLocalMode(t *testing.T) {
	primary := []HostInterface{
		{Name: "brvlan225", Type: "bridge", IPv6Mode: "none"},
		{Name: "br0", Type: "bridge", IPv6Mode: "dhcp"},
		{Name: "brstatic", Type: "bridge", IPv6Mode: "static"},
	}
	secondary := []HostInterface{
		{Name: "brvlan225", IPv6: "fe80::3648:edff:fef3:417d/64", IPv6Mode: "link-local"},
		{Name: "br0", IPv6: "fe80::3648:edff:fef3:417c/64", IPv6Mode: "link-local"},
		{Name: "brstatic", IPv6: "fe80::3648:edff:fef3:417e/64", IPv6Mode: "link-local"},
	}

	result := mergeHostInterfaces(primary, secondary, false)

	if result[0].IPv6Mode != "link-local" {
		t.Fatalf("expected none mode with runtime link-local address to become link-local, got %s", result[0].IPv6Mode)
	}
	if result[1].IPv6Mode != "dhcp" {
		t.Fatalf("expected dhcp mode to remain dhcp, got %s", result[1].IPv6Mode)
	}
	if result[2].IPv6Mode != "static" {
		t.Fatalf("expected static mode to remain static, got %s", result[2].IPv6Mode)
	}
}

func TestFilterFallbackHostInterfacesHidesRuntimeNoise(t *testing.T) {
	items := []HostInterface{
		{Name: "lo", Type: "loopback"},
		{Name: "br0", Type: "bridge"},
		{Name: "em1", Type: "ethernet"},
		{Name: "vnet4", Type: "ethernet"},
		{Name: "tap0", Type: "ethernet"},
		{Name: "vethabc", Type: "ethernet"},
		{Name: "docker0", Type: "bridge"},
		{Name: "br-123456", Type: "bridge"},
		{Name: "em2.225", Type: "ethernet"},
		{Name: "idrac", Type: "ethernet"},
	}

	result := filterFallbackHostInterfaces(items)

	names := map[string]bool{}
	for _, item := range result {
		names[item.Name] = true
	}
	for _, name := range []string{"lo", "br0", "em1"} {
		if !names[name] {
			t.Fatalf("expected %s to remain in fallback list", name)
		}
	}
	for _, name := range []string{"vnet4", "tap0", "vethabc", "docker0", "br-123456", "em2.225", "idrac"} {
		if names[name] {
			t.Fatalf("expected %s to be filtered from fallback list", name)
		}
	}
}

func TestValidateRequestedAddressUniquenessRejectsDuplicateIPv4(t *testing.T) {
	existing := []HostInterface{{Name: "br0", IPv4: "172.18.0.11/24", IPv4Mode: "static"}}

	if err := validateRequestedAddressUniqueness("static", "172.18.0.11/25", "ipv4", "", existing); err == nil {
		t.Fatal("expected duplicate IPv4 address to be rejected")
	}
}

func TestValidateRequestedAddressUniquenessRejectsDuplicateIPv4Subnet(t *testing.T) {
	existing := []HostInterface{{Name: "br0", IPv4: "172.18.0.11/24", IPv4Mode: "static"}}

	if err := validateRequestedAddressUniqueness("static", "172.18.0.20/25", "ipv4", "", existing); err == nil {
		t.Fatal("expected overlapping IPv4 subnet to be rejected")
	}
}

func TestValidateRequestedAddressUniquenessRejectsDuplicateIPv6(t *testing.T) {
	existing := []HostInterface{{Name: "br6", IPv6: "2001:db8::10/64", IPv6Mode: "static"}}

	if err := validateRequestedAddressUniqueness("static", "2001:db8::10/80", "ipv6", "", existing); err == nil {
		t.Fatal("expected duplicate IPv6 address to be rejected")
	}
}

func TestValidateRequestedAddressUniquenessRejectsDuplicateIPv6Subnet(t *testing.T) {
	existing := []HostInterface{{Name: "br6", IPv6: "2001:db8::10/64", IPv6Mode: "static"}}

	if err := validateRequestedAddressUniqueness("static", "2001:db8::20/80", "ipv6", "", existing); err == nil {
		t.Fatal("expected overlapping IPv6 subnet to be rejected")
	}
}

func TestValidateRequestedAddressUniquenessAllowsDifferentSubnets(t *testing.T) {
	existing := []HostInterface{
		{Name: "br0", IPv4: "172.18.0.11/24", IPv4Mode: "static"},
		{Name: "br6", IPv6: "2001:db8::10/64", IPv6Mode: "static"},
	}

	if err := validateRequestedAddressUniqueness("static", "172.18.1.11/24", "ipv4", "", existing); err != nil {
		t.Fatalf("expected different IPv4 subnet to be allowed, got %v", err)
	}
	if err := validateRequestedAddressUniqueness("static", "2001:db8:1::10/64", "ipv6", "", existing); err != nil {
		t.Fatalf("expected different IPv6 subnet to be allowed, got %v", err)
	}
	if err := validateRequestedAddressUniqueness("dhcp", "", "ipv4", "", existing); err != nil {
		t.Fatalf("expected non-static mode to be allowed, got %v", err)
	}
}

func TestValidateRequestedAddressUniquenessAllowsBridgeSourceAddress(t *testing.T) {
	existing := []HostInterface{
		{Name: "eth0", IPv4: "192.168.51.48/24", IPv4Mode: "static"},
		{Name: "eth1", IPv4: "192.168.52.48/24", IPv4Mode: "static"},
	}
	if err := validateRequestedAddressUniqueness("static", "192.168.51.48/24", "ipv4", "eth0", existing); err != nil {
		t.Fatalf("expected bridge source IP to be reusable, got %v", err)
	}
	if err := validateRequestedAddressUniqueness("static", "192.168.52.48/24", "ipv4", "eth0", existing); err == nil {
		t.Fatal("expected non-source duplicate IP to be rejected")
	}
}

func TestValidateInterfaceGatewayRejectsOutOfSubnetGateway(t *testing.T) {
	if err := validateInterfaceGateway("static", "192.168.51.48/24", "192.168.52.254", "ipv4"); err == nil {
		t.Fatal("expected IPv4 gateway outside subnet to be rejected")
	}
	if err := validateInterfaceGateway("static", "2001:db8::10/64", "2001:db8:1::1", "ipv6"); err == nil {
		t.Fatal("expected IPv6 gateway outside subnet to be rejected")
	}
}

func TestValidateInterfaceGatewayAllowsSameSubnetGateway(t *testing.T) {
	if err := validateInterfaceGateway("static", "192.168.51.48/24", "192.168.51.254", "ipv4"); err != nil {
		t.Fatalf("expected IPv4 gateway in same subnet to be allowed, got %v", err)
	}
	if err := validateInterfaceGateway("static", "2001:db8::10/64", "2001:db8::1", "ipv6"); err != nil {
		t.Fatalf("expected IPv6 gateway in same subnet to be allowed, got %v", err)
	}
	if err := validateInterfaceGateway("dhcp", "", "192.168.52.254", "ipv4"); err != nil {
		t.Fatalf("expected non-static gateway validation to be skipped, got %v", err)
	}
}

func TestValidateInterfaceGatewayRejectsAddressAsGateway(t *testing.T) {
	if err := validateInterfaceGateway("static", "192.168.51.48/24", "192.168.51.48", "ipv4"); err == nil {
		t.Fatal("expected IPv4 gateway equal to address to be rejected")
	}
	if err := validateInterfaceGateway("static", "2001:db8::10/64", "2001:db8::10", "ipv6"); err == nil {
		t.Fatal("expected IPv6 gateway equal to address to be rejected")
	}
}

func TestUpdateIFCFGDNSReplacesExistingDNS(t *testing.T) {
	content := "DEVICE=br0\nBOOTPROTO=none\nDNS1=1.1.1.1\nDNS2=8.8.8.8\nONBOOT=yes\n"
	got := updateIFCFGDNS(content, []string{"192.168.50.5", "8.8.4.4"})
	expected := "DEVICE=br0\nBOOTPROTO=none\nONBOOT=yes\nDNS1=192.168.50.5\nDNS2=8.8.4.4\n"
	if got != expected {
		t.Fatalf("expected ifcfg dns update %q, got %q", expected, got)
	}
}

func TestValidateDNSServersRejectsInvalidAddress(t *testing.T) {
	if err := validateDNSServers([]string{"192.168.50.5", "bad-dns"}); err == nil {
		t.Fatal("expected invalid DNS server to be rejected")
	}
	if err := validateDNSServers([]string{"192.168.50.5", "2001:db8::53"}); err != nil {
		t.Fatalf("expected valid DNS servers to be allowed, got %v", err)
	}
}

func TestSplitDNSServersSeparatesAddressFamilies(t *testing.T) {
	ipv4, ipv6 := splitDNSServers([]string{"192.168.50.5", "2001:db8::53", "8.8.8.8"})
	if strings.Join(ipv4, ",") != "192.168.50.5,8.8.8.8" {
		t.Fatalf("unexpected IPv4 DNS list: %v", ipv4)
	}
	if strings.Join(ipv6, ",") != "2001:db8::53" {
		t.Fatalf("unexpected IPv6 DNS list: %v", ipv6)
	}
}

func TestHostInterfaceDeviceInUse(t *testing.T) {
	items := []HostInterface{
		{Name: "br0", Type: "bridge", BridgeDevice: "em1"},
		{Name: "br1", Type: "bridge", BridgeDevice: "em2"},
	}
	if !hostInterfaceDeviceInUse(items, "em1") {
		t.Fatal("expected em1 to be marked as in use")
	}
	if hostInterfaceDeviceInUse(items, "em3") {
		t.Fatal("expected em3 to be available")
	}
	if hostInterfaceDeviceInUse(items, "") {
		t.Fatal("expected empty device to be ignored")
	}
}

func TestBackupIFCFGIfExistsCreatesTimestampBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ifcfg-em1")
	content := []byte("DEVICE=em1\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("expected source ifcfg to be written, got %v", err)
	}
	if err := backupIFCFGIfExists(path); err != nil {
		t.Fatalf("expected backup to succeed, got %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "ifcfg-em1.*.bak"))
	if err != nil {
		t.Fatalf("expected backup glob to succeed, got %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one backup file, got %d", len(matches))
	}
	got, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("expected backup file to be readable, got %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("expected backup content %q, got %q", content, got)
	}
}

func TestShouldCleanupDeletedHostInterfaceLinkOnlyAllowsBridge(t *testing.T) {
	cases := []struct {
		name string
		item HostInterface
		want bool
	}{
		{name: "bridge", item: HostInterface{Name: "br-test", Type: "bridge"}, want: true},
		{name: "bridge case insensitive", item: HostInterface{Name: "br-test", Type: "BRIDGE"}, want: true},
		{name: "ethernet", item: HostInterface{Name: "eth0", Type: "ethernet"}, want: false},
		{name: "loopback", item: HostInterface{Name: "lo", Type: "loopback"}, want: false},
		{name: "missing name", item: HostInterface{Type: "bridge"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldCleanupDeletedHostInterfaceLink(tc.item); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
