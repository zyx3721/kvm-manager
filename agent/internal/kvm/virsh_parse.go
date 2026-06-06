package kvm

import (
	"runtime"
	"strconv"
	"strings"
)

func parseJSONInt64(value string, key string) int64 {
	needle := `"` + key + `":`
	idx := strings.Index(value, needle)
	if idx < 0 {
		return 0
	}
	rest := value[idx+len(needle):]
	rest = strings.TrimLeft(rest, " \t\r\n")
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	parsed, _ := strconv.ParseInt(rest[:end], 10, 64)
	return parsed
}

func parseTwoFloats(value string) (float64, float64) {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return 0, 0
	}
	first, _ := strconv.ParseFloat(fields[0], 64)
	second, _ := strconv.ParseFloat(fields[1], 64)
	return first, second
}

func normalizeState(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(s, "running"):
		return "running"
	case strings.Contains(s, "paused"):
		return "paused"
	case strings.Contains(s, "shut off") || strings.Contains(s, "shutoff"):
		return "stopped"
	case strings.Contains(s, "crashed"):
		return "error"
	default:
		return "unknown"
	}
}

func parseNodeCPU(info string) int {
	return parseNodeInt(info, "CPU(s):")
}

func parseNodeMemory(info string) int64 {
	kb := int64(parseNodeInt(info, "Memory size:"))
	return kb * 1024
}

func parseNodeText(info, prefix string) string {
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func configMemoryKiB(doc domainXML) (int64, int64) {
	current := doc.CurrentMemory.Value
	if current <= 0 {
		current = doc.Memory.Value
	}
	maximum := doc.Memory.Value
	if maximum <= 0 {
		maximum = current
	}
	return current, maximum
}

func parseNodeInt(info, prefix string) int {
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
			if len(fields) == 0 {
				return 0
			}
			value, _ := strconv.Atoi(fields[0])
			return value
		}
	}
	return 0
}

func kibToBytes(value int64) int64 { return value * 1024 }

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func firstLine(value string) string {
	line := strings.TrimSpace(strings.Split(value, "\n")[0])
	if line == "" && runtime.GOOS == "windows" {
		return "virsh unavailable on windows"
	}
	return line
}
