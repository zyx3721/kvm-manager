package kvm

import (
	"strconv"
	"strings"
)

func (p *VirshProvider) vmUptimeSeconds(name string, uuid string, status string) int64 {
	if status != "running" {
		return 0
	}
	for _, pattern := range []string{uuid, name} {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		out, err := p.output("pgrep", "-f", pattern)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			pid := strings.TrimSpace(line)
			if pid == "" || !isDigits(pid) {
				continue
			}
			if etime, err := p.output("ps", "-p", pid, "-o", "etime="); err == nil {
				if seconds := parseEtimeSeconds(etime); seconds > 0 {
					return seconds
				}
			}
		}
	}
	return 0
}

func parseEtimeSeconds(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	days := int64(0)
	if strings.Contains(value, "-") {
		parts := strings.SplitN(value, "-", 2)
		parsedDays, _ := strconv.ParseInt(parts[0], 10, 64)
		days = parsedDays
		value = parts[1]
	}
	parts := strings.Split(value, ":")
	var hours, minutes, seconds int64
	switch len(parts) {
	case 2:
		minutes, _ = strconv.ParseInt(parts[0], 10, 64)
		seconds, _ = strconv.ParseInt(parts[1], 10, 64)
	case 3:
		hours, _ = strconv.ParseInt(parts[0], 10, 64)
		minutes, _ = strconv.ParseInt(parts[1], 10, 64)
		seconds, _ = strconv.ParseInt(parts[2], 10, 64)
	default:
		return 0
	}
	return days*86400 + hours*3600 + minutes*60 + seconds
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
