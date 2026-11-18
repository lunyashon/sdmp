package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/regionname"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.DictRepository = (*DictRepo)(nil)

type DictRepo struct {
	pool *pgxpool.Pool
}

func NewDictRepo(pool *pgxpool.Pool) *DictRepo {
	return &DictRepo{pool: pool}
}

func (r *DictRepo) ListRegions(ctx context.Context) ([]entity.NamedRef, error) {
	return r.listNamed(ctx, `SELECT id, name FROM region ORDER BY name`)
}

func (r *DictRepo) ListOperators(ctx context.Context) ([]entity.NamedRef, error) {
	return r.listNamed(ctx, `SELECT id, name FROM operator ORDER BY id`)
}

func (r *DictRepo) ListStatuses(ctx context.Context) ([]entity.NamedRef, error) {
	return r.listNamed(ctx, `SELECT id, name FROM status ORDER BY id`)
}

func (r *DictRepo) ResolveRegion(ctx context.Context, alias string) (int, bool, error) {
	keys := regionname.LookupKeys(alias)
	if len(keys) == 0 {
		return 0, false, nil
	}
	var id int
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM (
			SELECT id FROM region WHERE lower(name) = ANY($1::text[])
			UNION ALL
			SELECT region_id FROM region_alias WHERE lower(alias) = ANY($1::text[])
		) t
		LIMIT 1
	`, lowerKeys(keys)).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("resolve region: %w", err)
	}
	return id, true, nil
}

func lowerKeys(keys []string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = strings.ToLower(strings.TrimSpace(k))
	}
	return out
}

func (r *DictRepo) ListOperatorGroups(ctx context.Context) ([]entity.OperatorGroup, error) {
	const q = `
		SELECT o.id, o.name,
			COALESCE(array_agg(a.alias ORDER BY a.alias) FILTER (WHERE a.alias IS NOT NULL), '{}')
		FROM operator o
		LEFT JOIN operator_alias a ON a.operator_id = o.id
		GROUP BY o.id, o.name
		ORDER BY o.id
	`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list operator groups: %w", err)
	}
	defer rows.Close()
	out := make([]entity.OperatorGroup, 0)
	for rows.Next() {
		var g entity.OperatorGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.Aliases); err != nil {
			return nil, err
		}
		if g.Aliases == nil {
			g.Aliases = []string{}
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	otherID := 0
	mapped := make(map[string]struct{}, 64)
	for _, g := range out {
		if g.Name == "ДРУГИЕ" {
			otherID = g.ID
		}
		if key := strings.ToLower(strings.TrimSpace(g.Name)); key != "" {
			mapped[key] = struct{}{}
		}
		for _, alias := range g.Aliases {
			if key := strings.ToLower(strings.TrimSpace(alias)); key != "" {
				mapped[key] = struct{}{}
			}
		}
	}
	if otherID == 0 {
		return out, nil
	}
	unmapped, err := r.listUnmappedOperatorRaws(ctx, otherID, mapped)
	if err != nil {
		return nil, err
	}
	if len(unmapped) > 0 {
		for i := range out {
			if out[i].ID == otherID {
				out[i].Unmapped = unmapped
				break
			}
		}
	}
	return out, nil
}

func (r *DictRepo) listUnmappedOperatorRaws(ctx context.Context, otherID int, mapped map[string]struct{}) ([]string, error) {
	const q = `
		SELECT DISTINCT operator_raw
		FROM lead_row
		WHERE operator_id = $1
		  AND operator_raw IS NOT NULL
		  AND operator_raw <> ''
		ORDER BY 1
		LIMIT 500
	`
	rows, err := r.pool.Query(ctx, q, otherID)
	if err != nil {
		return nil, fmt.Errorf("list unmapped operator raws: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		raw = strings.TrimSpace(raw)
		key := strings.ToLower(raw)
		if raw == "" {
			continue
		}
		if _, ok := mapped[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, raw)
		if len(out) >= 200 {
			break
		}
	}
	return out, rows.Err()
}

func (r *DictRepo) ResolveOperator(ctx context.Context, alias string) (int, bool, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return 0, false, nil
	}
	var id int
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM (
			SELECT id FROM operator WHERE lower(name) = lower($1)
			UNION ALL
			SELECT operator_id FROM operator_alias WHERE lower(alias) = lower($1)
		) t
		LIMIT 1
	`, alias).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("resolve operator: %w", err)
	}
	return id, true, nil
}

func (r *DictRepo) AddOperatorAlias(ctx context.Context, operatorID int, alias string) error {
	alias = strings.TrimSpace(alias)
	if operatorID <= 0 || alias == "" {
		return fmt.Errorf("operator alias is required")
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO operator_alias (alias, operator_id)
		SELECT $2, $1
		WHERE EXISTS (SELECT 1 FROM operator WHERE id = $1)
		ON CONFLICT (alias) DO UPDATE SET operator_id = EXCLUDED.operator_id
	`, operatorID, alias)
	if err != nil {
		return fmt.Errorf("add operator alias: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: operator id %d", domain.ErrNotFound, operatorID)
	}
	return nil
}

func (r *DictRepo) RemoveOperatorAlias(ctx context.Context, operatorID int, alias string) error {
	alias = strings.TrimSpace(alias)
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM operator_alias
		WHERE operator_id = $1 AND lower(alias) = lower($2)
	`, operatorID, alias)
	if err != nil {
		return fmt.Errorf("remove operator alias: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: alias %q", domain.ErrNotFound, alias)
	}
	return nil
}

func (r *DictRepo) OperatorByName(ctx context.Context, name string) (int, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false, nil
	}
	var id int
	err := r.pool.QueryRow(ctx, `SELECT id FROM operator WHERE name = $1`, name).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("operator by name: %w", err)
	}
	return id, true, nil
}

func (r *DictRepo) listNamed(ctx context.Context, q string) ([]entity.NamedRef, error) {
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list dict: %w", err)
	}
	defer rows.Close()
	out := make([]entity.NamedRef, 0)
	for rows.Next() {
		var item entity.NamedRef
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
