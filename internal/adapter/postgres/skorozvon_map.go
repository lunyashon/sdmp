package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.SkorozvonMapRepository = (*SkorozvonMapRepo)(nil)

type SkorozvonMapRepo struct {
	pool *pgxpool.Pool
}

func NewSkorozvonMapRepo(pool *pgxpool.Pool) *SkorozvonMapRepo {
	return &SkorozvonMapRepo{pool: pool}
}

func (r *SkorozvonMapRepo) ReplaceAll(ctx context.Context, rows []entity.SkorozvonMap) error {
	if len(rows) == 0 {
		return fmt.Errorf("replace skorozvon_maps: empty")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin skorozvon_maps txn: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	fieldIDs := make([]int64, 0, len(rows))
	optionIDs := make([]int64, 0, len(rows))
	const q = `
		INSERT INTO skorozvon_maps (field_id, option_id, name, bitrix_code, field_type, lead_type, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7)
		ON CONFLICT (field_id, option_id) DO UPDATE SET
			name = EXCLUDED.name,
			field_type = EXCLUDED.field_type,
			lead_type = EXCLUDED.lead_type,
			updated_at = EXCLUDED.updated_at,
			bitrix_code = COALESCE(NULLIF(EXCLUDED.bitrix_code, ''), skorozvon_maps.bitrix_code)
	`
	for _, row := range rows {
		at := row.UpdatedAt
		if at.IsZero() {
			at = now
		}
		if _, err := tx.Exec(ctx, q, row.FieldID, row.OptionID, row.Name, row.BitrixCode, row.FieldType, row.LeadType, at); err != nil {
			return fmt.Errorf("upsert skorozvon_maps field_id=%d option_id=%d: %w", row.FieldID, row.OptionID, err)
		}
		fieldIDs = append(fieldIDs, row.FieldID)
		optionIDs = append(optionIDs, row.OptionID)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM skorozvon_maps m
		WHERE NOT EXISTS (
			SELECT 1
			FROM unnest($1::bigint[], $2::bigint[]) AS n(field_id, option_id)
			WHERE n.field_id = m.field_id AND n.option_id = m.option_id
		)
	`, fieldIDs, optionIDs); err != nil {
		return fmt.Errorf("prune skorozvon_maps: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit skorozvon_maps: %w", err)
	}
	return nil
}

func (r *SkorozvonMapRepo) Get(ctx context.Context, fieldID, optionID int64) (entity.SkorozvonMap, bool, error) {
	var row entity.SkorozvonMap
	err := r.pool.QueryRow(ctx, `
		SELECT field_id, option_id, name, COALESCE(bitrix_code, ''), COALESCE(source_key, ''), COALESCE(field_type, ''), COALESCE(lead_type, ''), updated_at
		FROM skorozvon_maps
		WHERE field_id = $1 AND option_id = $2
	`, fieldID, optionID).Scan(&row.FieldID, &row.OptionID, &row.Name, &row.BitrixCode, &row.SourceKey, &row.FieldType, &row.LeadType, &row.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.SkorozvonMap{}, false, nil
		}
		return entity.SkorozvonMap{}, false, fmt.Errorf("get skorozvon_maps: %w", err)
	}
	return row, true, nil
}

func (r *SkorozvonMapRepo) ListFields(ctx context.Context) ([]entity.SkorozvonMap, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT field_id, option_id, name, COALESCE(bitrix_code, ''), COALESCE(source_key, ''), COALESCE(field_type, ''), COALESCE(lead_type, ''), updated_at
		FROM skorozvon_maps
		WHERE option_id = 0
		  AND (lead_type = 'lead' OR COALESCE(lead_type, '') = '')
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list skorozvon fields: %w", err)
	}
	defer rows.Close()
	out := make([]entity.SkorozvonMap, 0)
	for rows.Next() {
		var row entity.SkorozvonMap
		if err := rows.Scan(&row.FieldID, &row.OptionID, &row.Name, &row.BitrixCode, &row.SourceKey, &row.FieldType, &row.LeadType, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan skorozvon field: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *SkorozvonMapRepo) SetBitrixCodes(ctx context.Context, rows []entity.SkorozvonMap) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin skorozvon bitrix codes: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, row := range rows {
		if _, err := tx.Exec(ctx, `
			UPDATE skorozvon_maps
			SET bitrix_code = NULLIF($2, ''), updated_at = now()
			WHERE field_id = $1 AND option_id = 0
		`, row.FieldID, row.BitrixCode); err != nil {
			return fmt.Errorf("set skorozvon bitrix_code field_id=%d: %w", row.FieldID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit skorozvon bitrix codes: %w", err)
	}
	return nil
}
