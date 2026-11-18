package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.BitrixMapRepository = (*BitrixMapRepo)(nil)

type BitrixMapRepo struct {
	pool *pgxpool.Pool
}

func NewBitrixMapRepo(pool *pgxpool.Pool) *BitrixMapRepo {
	return &BitrixMapRepo{pool: pool}
}

func (r *BitrixMapRepo) ReplaceFields(ctx context.Context, rows []entity.BitrixLeadField) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin bitrix fields: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM bitrix_lead_field`); err != nil {
		return fmt.Errorf("clear bitrix fields: %w", err)
	}
	now := time.Now()
	for _, row := range rows {
		at := row.UpdatedAt
		if at.IsZero() {
			at = now
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO bitrix_lead_field (code, title, type, writable, updated_at)
			VALUES ($1, $2, $3, $4, $5)
		`, row.Code, row.Title, row.Type, row.Writable, at); err != nil {
			return fmt.Errorf("insert bitrix field %s: %w", row.Code, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit bitrix fields: %w", err)
	}
	return nil
}

func (r *BitrixMapRepo) ReplaceStatuses(ctx context.Context, entityID string, rows []entity.BitrixStatus) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin bitrix statuses: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM bitrix_status WHERE entity_id = $1`, entityID); err != nil {
		return fmt.Errorf("clear bitrix statuses: %w", err)
	}
	now := time.Now()
	for _, row := range rows {
		at := row.UpdatedAt
		if at.IsZero() {
			at = now
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO bitrix_status (entity_id, status_id, name, sort, updated_at)
			VALUES ($1, $2, $3, $4, $5)
		`, entityID, row.StatusID, row.Name, row.Sort, at); err != nil {
			return fmt.Errorf("insert bitrix status %s/%s: %w", entityID, row.StatusID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit bitrix statuses: %w", err)
	}
	return nil
}

func (r *BitrixMapRepo) ReplaceMaps(ctx context.Context, rows []entity.BitrixLeadMap) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin bitrix maps: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM bitrix_lead_map`); err != nil {
		return fmt.Errorf("clear bitrix maps: %w", err)
	}
	now := time.Now()
	for _, row := range rows {
		at := row.UpdatedAt
		if at.IsZero() {
			at = now
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO bitrix_lead_map (source_key, bitrix_code, fixed_value, updated_at)
			VALUES ($1, $2, $3, $4)
		`, row.SourceKey, row.BitrixCode, row.FixedValue, at); err != nil {
			return fmt.Errorf("insert bitrix map %s: %w", row.SourceKey, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit bitrix maps: %w", err)
	}
	return nil
}

func (r *BitrixMapRepo) ListFields(ctx context.Context) ([]entity.BitrixLeadField, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT code, title, type, writable, updated_at
		FROM bitrix_lead_field
		ORDER BY writable DESC, title, code
	`)
	if err != nil {
		return nil, fmt.Errorf("list bitrix fields: %w", err)
	}
	defer rows.Close()
	out := make([]entity.BitrixLeadField, 0)
	for rows.Next() {
		var row entity.BitrixLeadField
		if err := rows.Scan(&row.Code, &row.Title, &row.Type, &row.Writable, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan bitrix field: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *BitrixMapRepo) ListStatuses(ctx context.Context, entityID string) ([]entity.BitrixStatus, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT entity_id, status_id, name, sort, updated_at
		FROM bitrix_status
		WHERE entity_id = $1
		ORDER BY sort, name
	`, entityID)
	if err != nil {
		return nil, fmt.Errorf("list bitrix statuses: %w", err)
	}
	defer rows.Close()
	out := make([]entity.BitrixStatus, 0)
	for rows.Next() {
		var row entity.BitrixStatus
		if err := rows.Scan(&row.EntityID, &row.StatusID, &row.Name, &row.Sort, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan bitrix status: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *BitrixMapRepo) ListMaps(ctx context.Context) ([]entity.BitrixLeadMap, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT source_key, bitrix_code, COALESCE(fixed_value, ''), updated_at
		FROM bitrix_lead_map
		ORDER BY source_key
	`)
	if err != nil {
		return nil, fmt.Errorf("list bitrix maps: %w", err)
	}
	defer rows.Close()
	out := make([]entity.BitrixLeadMap, 0)
	for rows.Next() {
		var row entity.BitrixLeadMap
		if err := rows.Scan(&row.SourceKey, &row.BitrixCode, &row.FixedValue, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan bitrix map: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
