package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

type SnapshotAnnotationInput struct {
	DisplayName string
	Description string
	Tags        []string
}

type snapshotAnnotation struct {
	DisplayName string
	Description string
	Tags        []string
}

func (s *Store) ApplySnapshotAnnotations(ctx context.Context, snapshots []domain.Snapshot) ([]domain.Snapshot, error) {
	if len(snapshots) == 0 {
		return snapshots, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT agent_id::text, vm_name, snapshot_name, display_name, description, tags FROM snapshot_annotations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	annotations := map[string]snapshotAnnotation{}
	for rows.Next() {
		var agentID, vmName, snapshotName, displayName, description string
		var tagsBytes []byte
		if err := rows.Scan(&agentID, &vmName, &snapshotName, &displayName, &description, &tagsBytes); err != nil {
			return nil, err
		}
		var tags []string
		_ = json.Unmarshal(tagsBytes, &tags)
		annotations[snapshotAnnotationKey(agentID, vmName, snapshotName)] = snapshotAnnotation{DisplayName: displayName, Description: description, Tags: tags}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range snapshots {
		annotation, ok := annotations[snapshotAnnotationKey(snapshots[index].HostID, snapshots[index].VMName, snapshots[index].Name)]
		if !ok {
			continue
		}
		snapshots[index].DisplayName = annotation.DisplayName
		snapshots[index].Description = annotation.Description
		snapshots[index].Tags = annotation.Tags
	}
	return snapshots, nil
}

func (s *Store) UpsertSnapshotAnnotation(ctx context.Context, snapshot domain.Snapshot, input SnapshotAnnotationInput, userID string) (domain.Snapshot, error) {
	tagsBytes, err := json.Marshal(normalizeTags(input.Tags))
	if err != nil {
		return domain.Snapshot{}, err
	}
	var displayName, description string
	var storedTags []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO snapshot_annotations(agent_id, vm_name, snapshot_name, display_name, description, tags, created_by, updated_by)
		VALUES($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (agent_id, vm_name, snapshot_name)
		DO UPDATE SET display_name=EXCLUDED.display_name, description=EXCLUDED.description, tags=EXCLUDED.tags, updated_by=EXCLUDED.updated_by, updated_at=now()
		RETURNING display_name, description, tags
	`, snapshot.HostID, snapshot.VMName, snapshot.Name, strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.Description), tagsBytes, userID).Scan(&displayName, &description, &storedTags)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var tags []string
	_ = json.Unmarshal(storedTags, &tags)
	snapshot.DisplayName = displayName
	snapshot.Description = description
	snapshot.Tags = tags
	return snapshot, nil
}

func (s *Store) GetSnapshotAnnotation(ctx context.Context, agentID, vmName, snapshotName string) (SnapshotAnnotationInput, error) {
	var input SnapshotAnnotationInput
	var tagsBytes []byte
	err := s.pool.QueryRow(ctx, `SELECT display_name, description, tags FROM snapshot_annotations WHERE agent_id=$1 AND vm_name=$2 AND snapshot_name=$3`, agentID, vmName, snapshotName).Scan(&input.DisplayName, &input.Description, &tagsBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return SnapshotAnnotationInput{}, ErrNotFound
	}
	if err != nil {
		return SnapshotAnnotationInput{}, err
	}
	_ = json.Unmarshal(tagsBytes, &input.Tags)
	return input, nil
}

func snapshotAnnotationKey(agentID, vmName, snapshotName string) string {
	return agentID + "\x00" + vmName + "\x00" + snapshotName
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}
