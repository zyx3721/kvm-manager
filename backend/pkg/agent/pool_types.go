package agent

type NetworkPool struct {
	Name           string                `json:"name"`
	State          string                `json:"state"`
	Autostart      bool                  `json:"autostart"`
	Bridge         string                `json:"bridge"`
	Forward        string                `json:"forward"`
	Subnet         string                `json:"subnet"`
	DHCP           bool                  `json:"dhcp"`
	DHCPStart      string                `json:"dhcpStart"`
	DHCPEnd        string                `json:"dhcpEnd"`
	FixedAddresses []NetworkFixedAddress `json:"fixedAddresses"`
	OpenVSwitch    bool                  `json:"openVSwitch"`
}

type NetworkFixedAddress struct {
	Address string `json:"address"`
	MAC     string `json:"mac"`
}

type NetworkPoolCreateRequest struct {
	Name         string `json:"name"`
	Subnet       string `json:"subnet"`
	DHCP         bool   `json:"dhcp"`
	FixedAddress bool   `json:"fixedAddress"`
	Type         string `json:"type"`
	Bridge       string `json:"bridge"`
	OpenVSwitch  bool   `json:"openVSwitch"`
}
