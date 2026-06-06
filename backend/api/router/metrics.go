package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type metricSeriesResponse struct {
	Range  string `json:"range"`
	Bucket string `json:"bucket"`
	Items  any    `json:"items"`
}

func (r *router) handleHostMetrics(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "hosts.read") {
		return
	}
	agentID := normalizeHostMetricAgentID(strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/metrics/hosts/"), "/"))
	if strings.Contains(agentID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	rangeLabel, since, until, bucket, ok := metricRange(req)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_metric_range", "时间范围无效")
		return
	}
	items, err := r.store.ListHostMetricSeries(req.Context(), agentID, since, until, bucket)
	if err != nil {
		r.logger.Error("list host metric series failed", "error", err)
		writeError(w, http.StatusInternalServerError, "list_metrics_failed", "读取指标趋势失败")
		return
	}
	writeJSON(w, http.StatusOK, metricSeriesResponse{Range: rangeLabel, Bucket: bucket.String(), Items: items})
}

func normalizeHostMetricAgentID(agentID string) string {
	if strings.EqualFold(agentID, "all") {
		return ""
	}
	return agentID
}

func (r *router) handleVMMetrics(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "vms.read") {
		return
	}
	vmID := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/metrics/vms/"), "/")
	if vmID == "" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	rangeLabel, since, until, bucket, ok := metricRange(req)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_metric_range", "时间范围无效")
		return
	}
	items, err := r.store.ListVMMetricSeries(req.Context(), vmID, since, until, bucket)
	if err != nil {
		r.logger.Error("list vm metric series failed", "error", err)
		writeError(w, http.StatusInternalServerError, "list_metrics_failed", "读取指标趋势失败")
		return
	}
	writeJSON(w, http.StatusOK, metricSeriesResponse{Range: rangeLabel, Bucket: bucket.String(), Items: items})
}

func metricRange(req *http.Request) (string, time.Time, time.Time, time.Duration, bool) {
	now := time.Now().UTC()
	query := req.URL.Query()
	if strings.EqualFold(strings.TrimSpace(query.Get("range")), "custom") {
		start, errStart := parseMetricTime(query.Get("start"))
		end, errEnd := parseMetricTime(query.Get("end"))
		if errStart != nil || errEnd != nil || !start.Before(end) {
			return "", time.Time{}, time.Time{}, 0, false
		}
		return "custom", start, end, metricBucket(end.Sub(start)), true
	}
	switch strings.ToLower(strings.TrimSpace(query.Get("range"))) {
	case "24h":
		return "24h", now.Add(-24 * time.Hour), now, 30 * time.Minute, true
	case "7d":
		return "7d", now.Add(-7 * 24 * time.Hour), now, time.Hour, true
	case "30d":
		return "30d", now.Add(-30 * 24 * time.Hour), now, 24 * time.Hour, true
	default:
		return "1h", now.Add(-time.Hour), now, time.Minute, true
	}
}

func parseMetricTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("invalid metric time")
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := time.Parse("2006-01-02T15:04", trimmed); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid metric time")
}

func metricBucket(window time.Duration) time.Duration {
	switch {
	case window <= 24*time.Hour:
		return time.Minute
	case window <= 7*24*time.Hour:
		return 30 * time.Minute
	case window <= 30*24*time.Hour:
		return time.Hour
	default:
		return 24 * time.Hour
	}
}
