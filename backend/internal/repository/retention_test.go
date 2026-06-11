package repository

import (
	"strings"
	"testing"
)

func TestLogRetentionUsesCreatedAtForAllLogTables(t *testing.T) {
	queries := map[string]string{
		"tasks":      deleteTasksBeforeSQL,
		"audit_logs": deleteAuditLogsBeforeSQL,
		"alerts":     deleteAlertsBeforeSQL,
	}
	for table, query := range queries {
		if !strings.Contains(query, "created_at < $1") {
			t.Fatalf("%s retention SQL must use created_at: %s", table, query)
		}
		if strings.Contains(query, "status") || strings.Contains(query, "finished_at") || strings.Contains(query, "resolved_at") || strings.Contains(query, "last_seen_at") {
			t.Fatalf("%s retention SQL must not use status or lifecycle timestamps: %s", table, query)
		}
	}
}
