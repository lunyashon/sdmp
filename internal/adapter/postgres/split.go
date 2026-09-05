package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.SplitRepository = (*SplitRepo)(nil)

type SplitRepo struct {
	pool *pgxpool.Pool
}

func NewSplitRepo(pool *pgxpool.Pool) *SplitRepo {
	return &SplitRepo{pool: pool}
}

func (r *SplitRepo) List(ctx context.Context) ([]entity.SourceSplit, error) {
	return r.load(ctx, 0)
}

func (r *SplitRepo) Resolve(ctx context.Context, sourceID, operatorID int) (entity.SplitResolve, bool, error) {
	var out entity.SplitResolve
	err := r.pool.QueryRow(ctx, `
		SELECT
			r.child_source_id,
			COALESCE(r.cold_label, ''),
			COALESCE(r.base_label, ''),
			r.test_child_source_id,
			COALESCE(t.cold_label, ''),
			COALESCE(t.base_label, ''),
			s.teg_active,
			COALESCE(s.teg_expected_value, '')
		FROM source_split r
		JOIN source s ON s.id = r.source_id
		LEFT JOIN source t ON t.id = r.test_child_source_id
		WHERE r.source_id = $1 AND r.operator_id = $2
		LIMIT 1
	`, sourceID, operatorID).Scan(
		&out.ChildSourceID,
		&out.ColdLabel,
		&out.BaseLabel,
		&out.TestChildSourceID,
		&out.TestColdLabel,
		&out.TestBaseLabel,
		&out.TegActive,
		&out.TegExpectedValue,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.SplitResolve{}, false, nil
		}
		return entity.SplitResolve{}, false, fmt.Errorf("resolve split: %w", err)
	}
	out.OperatorID = operatorID
	return out, true, nil
}

func (r *SplitRepo) Get(ctx context.Context, sourceID int) (entity.SourceSplit, error) {
	items, err := r.load(ctx, sourceID)
	if err != nil {
		return entity.SourceSplit{}, err
	}
	if len(items) == 0 {
		return entity.SourceSplit{}, fmt.Errorf("%w: source id %d", domain.ErrNotFound, sourceID)
	}
	return items[0], nil
}

func (r *SplitRepo) Replace(ctx context.Context, in entity.SourceSplit) (entity.SourceSplit, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return entity.SourceSplit{}, fmt.Errorf("begin replace split: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE source
		SET teg_active = $2, teg_expected_value = NULLIF($3, '')
		WHERE id = $1 AND parent_id IS NULL
	`, in.SourceID, in.TegActive, in.TegExpectedValue)
	if err != nil {
		return entity.SourceSplit{}, fmt.Errorf("update source teg: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return entity.SourceSplit{}, fmt.Errorf("%w: source id %d", domain.ErrNotFound, in.SourceID)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM source_split WHERE source_id = $1`, in.SourceID); err != nil {
		return entity.SourceSplit{}, fmt.Errorf("clear source split: %w", err)
	}

	for i := range in.Rules {
		rule := &in.Rules[i]
		if err := tx.QueryRow(ctx, `
			INSERT INTO source_split (source_id, operator_id, child_source_id, test_child_source_id, cold_label, base_label)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))
			RETURNING id
		`, in.SourceID, rule.OperatorID, rule.ChildSourceID, rule.TestChildSourceID, rule.ColdLabel, rule.BaseLabel).Scan(&rule.ID); err != nil {
			return entity.SourceSplit{}, wrapSplitFK(err, "insert source split")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.SourceSplit{}, fmt.Errorf("commit replace split: %w", err)
	}
	return r.Get(ctx, in.SourceID)
}

func (r *SplitRepo) load(ctx context.Context, sourceID int) ([]entity.SourceSplit, error) {
	q := `
		SELECT s.id, s.teg_active, COALESCE(s.teg_expected_value, ''),
			r.id, r.operator_id, r.child_source_id, r.test_child_source_id,
			COALESCE(NULLIF(r.cold_label, ''), COALESCE(c.cold_label, '')),
			COALESCE(NULLIF(r.base_label, ''), COALESCE(c.base_label, ''))
		FROM source s
		LEFT JOIN source_split r ON r.source_id = s.id
		LEFT JOIN source c ON c.id = r.child_source_id
		WHERE s.parent_id IS NULL
	`
	args := []any{}
	if sourceID > 0 {
		q += ` AND s.id = $1`
		args = append(args, sourceID)
	}
	q += ` ORDER BY s.id, r.id`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list splits: %w", err)
	}
	defer rows.Close()

	index := map[int]int{}
	out := make([]entity.SourceSplit, 0)
	for rows.Next() {
		var (
			sourceID        int
			tegActive       bool
			tegValue        string
			ruleID          *int
			operatorID      *int
			childID         *int
			testID          *int
			cold            string
			base            string
		)
		if err := rows.Scan(&sourceID, &tegActive, &tegValue, &ruleID, &operatorID, &childID, &testID, &cold, &base); err != nil {
			return nil, err
		}
		i, ok := index[sourceID]
		if !ok {
			i = len(out)
			index[sourceID] = i
			out = append(out, entity.SourceSplit{
				SourceID:         sourceID,
				TegActive:        tegActive,
				TegExpectedValue: tegValue,
				Rules:            []entity.SplitRule{},
			})
		}
		if ruleID == nil || operatorID == nil || childID == nil {
			continue
		}
		out[i].Rules = append(out[i].Rules, entity.SplitRule{
			ID:                *ruleID,
			OperatorID:        *operatorID,
			ChildSourceID:     *childID,
			TestChildSourceID: testID,
			ColdLabel:         cold,
			BaseLabel:         base,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func wrapSplitFK(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23505") {
		return fmt.Errorf("%w: invalid split rule", domain.ErrInvalidInput)
	}
	return fmt.Errorf("%s: %w", op, err)
}
