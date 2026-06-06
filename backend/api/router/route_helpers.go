package router

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func decodeJSONBody(w http.ResponseWriter, req *http.Request, target any) error {
	return decodeJSONBodyLimit(w, req, 1<<20, target)
}

func decodeJSONBodyLimit(w http.ResponseWriter, req *http.Request, maxBytes int64, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, req.Body, maxBytes))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func parseVMPath(path string) (id string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/vms/"), "/")
	if trimmed == "" {
		return "", "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 {
		return parts[0], "", true
	}
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func parseSnapshotPath(path string) (id string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/snapshots/"), "/")
	if trimmed == "" {
		return "", "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func mapVMAction(action string) (string, bool) {
	switch action {
	case "start", "reboot":
		return action, true
	case "force-reboot":
		return "reset", true
	case "pause":
		return "suspend", true
	case "resume":
		return "resume", true
	case "stop":
		return "shutdown", true
	case "force-stop":
		return "destroy", true
	case "shutdown":
		return "shutdown", true
	case "force-shutdown":
		return "destroy", true
	case "delete":
		return "delete", true
	case "force-delete":
		return "force-delete", true
	default:
		return "", false
	}
}

func parseLimit(req *http.Request, fallback int) int {
	raw := strings.TrimSpace(req.URL.Query().Get("limit"))
	if raw == "" {
		return fallback
	}
	if strings.EqualFold(raw, "all") {
		return 0
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return fallback
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func parseOffset(req *http.Request, limit int) int {
	if limit <= 0 {
		return 0
	}
	if rawOffset := strings.TrimSpace(req.URL.Query().Get("offset")); rawOffset != "" {
		offset, err := strconv.Atoi(rawOffset)
		if err == nil && offset > 0 {
			return offset
		}
	}
	page, err := strconv.Atoi(req.URL.Query().Get("page"))
	if err != nil || page <= 1 {
		return 0
	}
	return (page - 1) * limit
}
