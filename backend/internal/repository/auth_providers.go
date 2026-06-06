package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

func (s *Store) ListAuthProviders(ctx context.Context) ([]domain.AuthProvider, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, name, enabled, config, created_at, updated_at
		FROM auth_providers
		ORDER BY CASE id WHEN 'ldap' THEN 1 ELSE 9 END, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AuthProvider, 0)
	for rows.Next() {
		var item domain.AuthProvider
		if err := rows.Scan(&item.ID, &item.Type, &item.Name, &item.Enabled, &item.Config, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListEnabledPublicAuthProviders(ctx context.Context) ([]domain.PublicAuthProvider, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, name, enabled
		FROM auth_providers
		WHERE enabled=true
		ORDER BY CASE id WHEN 'ldap' THEN 1 ELSE 9 END, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PublicAuthProvider, 0)
	for rows.Next() {
		var item domain.PublicAuthProvider
		if err := rows.Scan(&item.ID, &item.Type, &item.Name, &item.Enabled); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetAuthProvider(ctx context.Context, id string) (domain.AuthProvider, error) {
	var item domain.AuthProvider
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, name, enabled, config, created_at, updated_at
		FROM auth_providers WHERE id=$1
	`, id).Scan(&item.ID, &item.Type, &item.Name, &item.Enabled, &item.Config, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AuthProvider{}, ErrNotFound
	}
	return item, err
}

func (s *Store) UpsertAuthProvider(ctx context.Context, id, providerType, name string, enabled bool, config any) (domain.AuthProvider, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return domain.AuthProvider{}, err
	}
	var item domain.AuthProvider
	err = s.pool.QueryRow(ctx, `
		INSERT INTO auth_providers(id, type, name, enabled, config)
		VALUES($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET type=EXCLUDED.type, name=EXCLUDED.name, enabled=EXCLUDED.enabled, config=EXCLUDED.config, updated_at=now()
		RETURNING id, type, name, enabled, config, created_at, updated_at
	`, id, providerType, name, enabled, configBytes).Scan(&item.ID, &item.Type, &item.Name, &item.Enabled, &item.Config, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}
