package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.CrmcallsProjectRepository = (*CrmcallsProjectRepo)(nil)

const crmcallsProjectSelect = `
	id, name, COALESCE(status, ''), last_date_request, last_request
`

type CrmcallsProjectRepo struct {
	pool *pgxpool.Pool
}

func NewCrmcallsProjectRepo(pool *pgxpool.Pool) *CrmcallsProjectRepo {
	return &CrmcallsProjectRepo{pool: pool}
}

func (r *CrmcallsProjectRepo) List(ctx context.Context) ([]entity.CrmcallsProject, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+crmcallsProjectSelect+`
		FROM crmcalls_project
		ORDER BY name, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list crmcalls projects: %w", err)
	}
	defer rows.Close()
	out := make([]entity.CrmcallsProject, 0)
	for rows.Next() {
		item, err := scanCrmcallsProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *CrmcallsProjectRepo) Get(ctx context.Context, id string) (entity.CrmcallsProject, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return entity.CrmcallsProject{}, fmt.Errorf("%w: crmcalls project id is required", domain.ErrInvalidInput)
	}
	item, err := scanCrmcallsProject(r.pool.QueryRow(ctx, `
		SELECT `+crmcallsProjectSelect+`
		FROM crmcalls_project
		WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.CrmcallsProject{}, fmt.Errorf("%w: crmcalls project %s", domain.ErrNotFound, id)
		}
		return entity.CrmcallsProject{}, fmt.Errorf("get crmcalls project: %w", err)
	}
	return item, nil
}

func (r *CrmcallsProjectRepo) Upsert(ctx context.Context, items []entity.CrmcallsProject) error {
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = id
		}
		if _, err := r.pool.Exec(ctx, `
			INSERT INTO crmcalls_project (id, name, status, updated_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				status = EXCLUDED.status,
				updated_at = now()
		`, id, name, strings.TrimSpace(item.Status)); err != nil {
			return fmt.Errorf("upsert crmcalls project: %w", err)
		}
	}
	return nil
}

func (r *CrmcallsProjectRepo) Ensure(ctx context.Context, id, name string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("crmcalls project id is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crmcalls_project (id, name, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (id) DO NOTHING
	`, id, name)
	if err != nil {
		return fmt.Errorf("ensure crmcalls project: %w", err)
	}
	return nil
}

func (r *CrmcallsProjectRepo) UpdateCheck(ctx context.Context, id string, at time.Time, ok bool, status, name string) (entity.CrmcallsProject, error) {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)
	name = strings.TrimSpace(name)
	item, err := scanCrmcallsProject(r.pool.QueryRow(ctx, `
		UPDATE crmcalls_project
		SET last_date_request = $2,
		    last_request = $3,
		    status = CASE WHEN $4 <> '' THEN $4 ELSE status END,
		    name = CASE WHEN $5 <> '' THEN $5 ELSE name END,
		    updated_at = now()
		WHERE id = $1
		RETURNING `+crmcallsProjectSelect+`
	`, id, at, ok, status, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.CrmcallsProject{}, fmt.Errorf("%w: crmcalls project %s", domain.ErrNotFound, id)
		}
		return entity.CrmcallsProject{}, fmt.Errorf("update crmcalls project check: %w", err)
	}
	return item, nil
}

type crmcallsProjectScanner interface {
	Scan(dest ...any) error
}

func scanCrmcallsProject(row crmcallsProjectScanner) (entity.CrmcallsProject, error) {
	var item entity.CrmcallsProject
	if err := row.Scan(&item.ID, &item.Name, &item.Status, &item.LastDateRequest, &item.LastRequest); err != nil {
		return entity.CrmcallsProject{}, err
	}
	return item, nil
}
