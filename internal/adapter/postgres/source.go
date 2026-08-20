package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.SourceRepository = (*SourceRepo)(nil)

type SourceRepo struct {
	pool *pgxpool.Pool
}

func NewSourceRepo(pool *pgxpool.Pool) *SourceRepo {
	return &SourceRepo{pool: pool}
}

func (r *SourceRepo) List(ctx context.Context) ([]entity.Source, error) {
	const q = `
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active
		FROM source
		ORDER BY id
	`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	out := make([]entity.Source, 0)
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (r *SourceRepo) GetByID(ctx context.Context, id int) (entity.Source, error) {
	const q = `
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active
		FROM source
		WHERE id = $1
	`
	src, err := scanSource(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Source{}, fmt.Errorf("%w: source id %d", domain.ErrNotFound, id)
		}
		return entity.Source{}, fmt.Errorf("get source by id: %w", err)
	}
	return src, nil
}

func (r *SourceRepo) GetByBitrixCode(ctx context.Context, code string) (entity.Source, error) {
	const q = `
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active
		FROM source
		WHERE bitrix_code = $1
		LIMIT 1
	`
	src, err := scanSource(r.pool.QueryRow(ctx, q, code))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Source{}, fmt.Errorf("%w: source %s", domain.ErrNotFound, code)
		}
		return entity.Source{}, fmt.Errorf("get source by code: %w", err)
	}
	return src, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSource(row rowScanner) (entity.Source, error) {
	var src entity.Source
	var bitrix, cold, base *string
	if err := row.Scan(
		&src.ID,
		&src.ParentID,
		&src.Name,
		&bitrix,
		&cold,
		&base,
		&src.IsActive,
	); err != nil {
		return entity.Source{}, err
	}
	src.BitrixCode = deref(bitrix)
	src.ColdLabel = deref(cold)
	src.BaseLabel = deref(base)
	return src, nil
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
