package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

func (s *Store) UpsertVMTemplateMark(ctx context.Context, agentID string, vmUUID string, name string, description string, createdBy string) (domain.VMTemplateMark, error) {
	var mark domain.VMTemplateMark
	err := s.pool.QueryRow(ctx, `
		INSERT INTO vm_template_marks(agent_id, vm_uuid, name, description, created_by)
		VALUES($1, $2, $3, $4, NULLIF($5, '')::uuid)
		ON CONFLICT (agent_id, vm_uuid) DO UPDATE SET
			name=EXCLUDED.name,
			description=EXCLUDED.description,
			updated_at=now()
		RETURNING id::text, agent_id::text, vm_uuid, name, description, COALESCE(created_by::text, ''), created_at, updated_at
	`, agentID, strings.TrimSpace(vmUUID), strings.TrimSpace(name), strings.TrimSpace(description), createdBy).Scan(
		&mark.ID,
		&mark.AgentID,
		&mark.VMUUID,
		&mark.Name,
		&mark.Description,
		&mark.CreatedBy,
		&mark.CreatedAt,
		&mark.UpdatedAt,
	)
	return mark, err
}

func (s *Store) DeleteVMTemplateMark(ctx context.Context, agentID string, vmUUID string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM vm_template_marks WHERE agent_id=$1 AND vm_uuid=$2`, agentID, strings.TrimSpace(vmUUID))
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListVMTemplateMarks(ctx context.Context) ([]domain.VMTemplateMark, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, agent_id::text, vm_uuid, name, description, COALESCE(created_by::text, ''), created_at, updated_at
		FROM vm_template_marks
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVMTemplateMarks(rows)
}

func (s *Store) ListVMTemplateMarksByAgent(ctx context.Context, agentID string) ([]domain.VMTemplateMark, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, agent_id::text, vm_uuid, name, description, COALESCE(created_by::text, ''), created_at, updated_at
		FROM vm_template_marks
		WHERE agent_id=$1
		ORDER BY updated_at DESC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVMTemplateMarks(rows)
}

func (s *Store) GetVMTemplateMark(ctx context.Context, agentID string, vmUUID string) (domain.VMTemplateMark, error) {
	var mark domain.VMTemplateMark
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, agent_id::text, vm_uuid, name, description, COALESCE(created_by::text, ''), created_at, updated_at
		FROM vm_template_marks
		WHERE agent_id=$1 AND vm_uuid=$2
	`, agentID, strings.TrimSpace(vmUUID)).Scan(
		&mark.ID,
		&mark.AgentID,
		&mark.VMUUID,
		&mark.Name,
		&mark.Description,
		&mark.CreatedBy,
		&mark.CreatedAt,
		&mark.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VMTemplateMark{}, ErrNotFound
	}
	return mark, err
}

type vmTemplateMarkRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanVMTemplateMarks(rows vmTemplateMarkRows) ([]domain.VMTemplateMark, error) {
	items := make([]domain.VMTemplateMark, 0)
	for rows.Next() {
		var mark domain.VMTemplateMark
		if err := rows.Scan(
			&mark.ID,
			&mark.AgentID,
			&mark.VMUUID,
			&mark.Name,
			&mark.Description,
			&mark.CreatedBy,
			&mark.CreatedAt,
			&mark.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, mark)
	}
	return items, rows.Err()
}
