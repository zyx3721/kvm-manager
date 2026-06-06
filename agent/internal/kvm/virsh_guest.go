package kvm

import (
	"encoding/json"
	"strings"
)

func (p *VirshProvider) detectOSType(name string, doc domainXML) string {
	if label := p.guestOSType(name); label != "" {
		return label
	}
	for _, value := range []string{doc.Metadata.OSInfo.Name, doc.Metadata.OSInfo.Version, doc.Metadata.OSInfo.ID} {
		if normalized := normalizeOSLabel(value); normalized != "" {
			return normalized
		}
	}
	if out, err := p.output("virsh", "--connect", p.libvirtURI, "dominfo", name); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "os:") || strings.HasPrefix(lower, "os type:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					if normalized := normalizeOSLabel(parts[1]); normalized != "" {
						return normalized
					}
				}
			}
		}
	}
	if guessed := guessOSFromName(doc.Name); guessed != "" {
		return guessed
	}
	if doc.OS.Type.Arch != "" {
		return doc.OS.Type.Arch + " virtualization"
	}
	return normalizeOSLabel(doc.OS.Type.Value)
}

func (p *VirshProvider) guestOSType(name string) string {
	out, err := p.output("virsh", "--connect", p.libvirtURI, "qemu-agent-command", name, `{"execute":"guest-get-osinfo"}`)
	if err != nil {
		return ""
	}
	var response struct {
		Return map[string]any `json:"return"`
	}
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		return ""
	}
	for _, key := range []string{"pretty-name", "name", "version", "id"} {
		if value, ok := response.Return[key].(string); ok {
			if normalized := normalizeOSLabel(value); normalized != "" {
				return normalized
			}
		}
	}
	return ""
}

func normalizeOSLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if lower == "hvm" || lower == "exe" || lower == "xen" || lower == "linux" {
		return ""
	}
	replacer := strings.NewReplacer("_", " ", "-", " ")
	label := strings.Join(strings.Fields(replacer.Replace(value)), " ")
	return label
}

func guessOSFromName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "win"):
		return "Windows"
	case strings.Contains(lower, "centos") || strings.Contains(lower, "ct"):
		return "CentOS"
	case strings.Contains(lower, "ubuntu"):
		return "Ubuntu"
	case strings.Contains(lower, "debian"):
		return "Debian"
	case strings.Contains(lower, "rocky"):
		return "Rocky Linux"
	case strings.Contains(lower, "alma"):
		return "AlmaLinux"
	case strings.Contains(lower, "fedora"):
		return "Fedora"
	default:
		return ""
	}
}

func (p *VirshProvider) primaryIP(name string) string {
	if out, err := p.output("virsh", "--connect", p.libvirtURI, "domifaddr", name, "--source", "agent"); err == nil {
		if ip := parseDomifaddrIP(out); ip != "" {
			return ip
		}
	}
	if out, err := p.output("virsh", "--connect", p.libvirtURI, "domifaddr", name, "--source", "lease"); err == nil {
		return parseDomifaddrIP(out)
	}
	return ""
}

func parseDomifaddrIP(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for _, field := range fields {
			if strings.Contains(field, ".") && strings.Contains(field, "/") {
				ip := strings.SplitN(field, "/", 2)[0]
				if isUsableGuestIPv4(ip) {
					return ip
				}
			}
		}
	}
	return ""
}

func isUsableGuestIPv4(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "127.") && !strings.HasPrefix(value, "169.254.")
}
