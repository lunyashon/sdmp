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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin skorozvon_maps txn: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM skorozvon_maps`); err != nil {
		return fmt.Errorf("clear skorozvon_maps: %w", err)
	}

	now := time.Now()
	const q = `
		INSERT INTO skorozvon_maps (field_id, option_id, name, field_type, lead_type, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6)
	`
	for _, row := range rows {
		at := row.UpdatedAt
		if at.IsZero() {
			at = now
		}
		if _, err := tx.Exec(ctx, q, row.FieldID, row.OptionID, row.Name, row.FieldType, row.LeadType, at); err != nil {
			return fmt.Errorf("insert skorozvon_maps field_id=%d option_id=%d: %w", row.FieldID, row.OptionID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit skorozvon_maps: %w", err)
	}
	return nil
}

func (r *SkorozvonMapRepo) Get(ctx context.Context, fieldID, optionID int64) (entity.SkorozvonMap, bool, error) {
	var row entity.SkorozvonMap
	err := r.pool.QueryRow(ctx, `
		SELECT field_id, option_id, name, COALESCE(field_type, ''), COALESCE(lead_type, ''), updated_at
		FROM skorozvon_maps
		WHERE field_id = $1 AND option_id = $2
	`, fieldID, optionID).Scan(&row.FieldID, &row.OptionID, &row.Name, &row.FieldType, &row.LeadType, &row.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.SkorozvonMap{}, false, nil
		}
		return entity.SkorozvonMap{}, false, fmt.Errorf("get skorozvon_maps: %w", err)
	}
	return row, true, nil
}
