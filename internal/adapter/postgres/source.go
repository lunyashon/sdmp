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
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active, idempotency_active, token
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
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active, idempotency_active, token
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
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active, idempotency_active, token
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

func (r *SourceRepo) GetByToken(ctx context.Context, token string) (entity.Source, error) {
	const q = `
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active, idempotency_active, token
		FROM source
		WHERE token = $1
		  AND token <> ''
		ORDER BY parent_id NULLS FIRST, id
		LIMIT 1
	`
	src, err := scanSource(r.pool.QueryRow(ctx, q, token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Source{}, fmt.Errorf("%w: token", domain.ErrNotFound)
		}
		return entity.Source{}, fmt.Errorf("get source by token: %w", err)
	}
	return src, nil
}

func (r *SourceRepo) ListFamily(ctx context.Context, rootID int) ([]entity.Source, error) {
	const q = `
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active, idempotency_active, token
		FROM source
		WHERE id = $1 OR parent_id = $1
		ORDER BY parent_id NULLS FIRST, id
	`
	rows, err := r.pool.Query(ctx, q, rootID)
	if err != nil {
		return nil, fmt.Errorf("list source family: %w", err)
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

func (r *SourceRepo) Create(ctx context.Context, src entity.Source) (entity.Source, error) {
	const q = `
		INSERT INTO source (parent_id, name, bitrix_code, cold_label, base_label, is_active)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6)
		RETURNING id, parent_id, name, bitrix_code, cold_label, base_label, is_active, idempotency_active, token
	`
	out, err := scanSource(r.pool.QueryRow(ctx, q,
		src.ParentID,
		src.Name,
		src.BitrixCode,
		src.ColdLabel,
		src.BaseLabel,
		src.IsActive,
	))
	if err != nil {
		return entity.Source{}, fmt.Errorf("create source: %w", err)
	}
	return out, nil
}

func (r *SourceRepo) SetActive(ctx context.Context, id int, active bool) (entity.Source, error) {
	const q = `
		UPDATE source
		SET is_active = $2
		WHERE id = $1 AND parent_id IS NULL
		RETURNING id, parent_id, name, bitrix_code, cold_label, base_label, is_active, idempotency_active, token
	`
	src, err := scanSource(r.pool.QueryRow(ctx, q, id, active))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Source{}, fmt.Errorf("%w: source id %d", domain.ErrNotFound, id)
		}
		return entity.Source{}, fmt.Errorf("set source active: %w", err)
	}
	return src, nil
}

func (r *SourceRepo) Delete(ctx context.Context, id int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete source: %w", err)
	}
	defer tx.Rollback(ctx)

	src, err := scanSource(tx.QueryRow(ctx, `
		SELECT id, parent_id, name, bitrix_code, cold_label, base_label, is_active, idempotency_active, token
		FROM source
		WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: source id %d", domain.ErrNotFound, id)
		}
		return fmt.Errorf("get source for delete: %w", err)
	}

	ids := []int{src.ID}
	if src.ParentID == nil {
		rows, err := tx.Query(ctx, `SELECT id FROM source WHERE parent_id = $1`, src.ID)
		if err != nil {
			return fmt.Errorf("list children for delete: %w", err)
		}
		for rows.Next() {
			var childID int
			if err := rows.Scan(&childID); err != nil {
				rows.Close()
				return fmt.Errorf("scan child for delete: %w", err)
			}
			ids = append(ids, childID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("list children for delete: %w", err)
		}
	}

	stmts := []struct {
		q  string
		op string
	}{
		{`DELETE FROM source_split WHERE source_id = ANY($1) OR child_source_id = ANY($1)`, "clear source split"},
		{`UPDATE source_split SET test_child_source_id = NULL WHERE test_child_source_id = ANY($1)`, "clear split test child"},
		{`DELETE FROM skorozvon_link_source WHERE source_id = ANY($1)`, "clear skorozvon link sources"},
		{`DELETE FROM skorozvon_link WHERE NOT EXISTS (SELECT 1 FROM skorozvon_link_source s WHERE s.link_id = skorozvon_link.id)`, "clear empty skorozvon links"},
		{`DELETE FROM source_upk WHERE source_id = ANY($1)`, "clear source upk"},
		{`DELETE FROM source_skorozvon WHERE source_id = ANY($1)`, "clear source skorozvon"},
		{`UPDATE lead_row SET child_source_id = NULL WHERE child_source_id = ANY($1)`, "detach lead child source"},
		{`UPDATE lead_row SET source_id = NULL WHERE source_id = ANY($1)`, "detach lead source"},
		{`DELETE FROM outbox WHERE source_id = ANY($1)`, "clear source outbox"},
		{`DELETE FROM source WHERE parent_id = ANY($1)`, "delete child sources"},
		{`DELETE FROM source WHERE id = ANY($1)`, "delete sources"},
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(ctx, stmt.q, ids); err != nil {
			return fmt.Errorf("%s: %w", stmt.op, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete source: %w", err)
	}
	return nil
}

func (r *SourceRepo) SetToken(ctx context.Context, id int, token string) error {
	var tokenVal any
	if token != "" {
		tokenVal = token
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE source
		SET token = $1
		WHERE id = $2
		  AND parent_id IS NULL
	`, tokenVal, id)
	if err != nil {
		return fmt.Errorf("set source token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: source id %d", domain.ErrNotFound, id)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSource(row rowScanner) (entity.Source, error) {
	var src entity.Source
	var bitrix, cold, base, token *string
	if err := row.Scan(
		&src.ID,
		&src.ParentID,
		&src.Name,
		&bitrix,
		&cold,
		&base,
		&src.IsActive,
		&src.IdempotencyActive,
		&token,
	); err != nil {
		return entity.Source{}, err
	}
	src.BitrixCode = deref(bitrix)
	src.ColdLabel = deref(cold)
	src.BaseLabel = deref(base)
	src.Token = deref(token)
	return src, nil
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
