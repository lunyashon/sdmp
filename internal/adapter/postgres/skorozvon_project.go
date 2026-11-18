package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.SkorozvonProjectRepository = (*SkorozvonProjectRepo)(nil)

type SkorozvonProjectRepo struct {
	pool *pgxpool.Pool
}

func NewSkorozvonProjectRepo(pool *pgxpool.Pool) *SkorozvonProjectRepo {
	return &SkorozvonProjectRepo{pool: pool}
}

func (r *SkorozvonProjectRepo) List(ctx context.Context) ([]entity.SkorozvonProject, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT uuid, project_id, name, created_at, last_date_request, last_request
		FROM skorozvon_project
		ORDER BY created_at, name
	`)
	if err != nil {
		return nil, fmt.Errorf("list skorozvon projects: %w", err)
	}
	defer rows.Close()

	out := make([]entity.SkorozvonProject, 0)
	for rows.Next() {
		p, err := scanSkorozvonProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const skorozvonProjectPageSize = 5

func (r *SkorozvonProjectRepo) ListPage(ctx context.Context, offset int, name string) ([]entity.SkorozvonProject, int, error) {
	if offset < 0 {
		offset = 0
	}
	name = strings.TrimSpace(name)

	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM skorozvon_project
		WHERE $1 = '' OR strpos(lower(name), lower($1)) > 0
	`, name).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count skorozvon projects: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT uuid, project_id, name, created_at, last_date_request, last_request
		FROM skorozvon_project
		WHERE $1 = '' OR strpos(lower(name), lower($1)) > 0
		ORDER BY created_at, name
		LIMIT $2 OFFSET $3
	`, name, skorozvonProjectPageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list skorozvon projects page: %w", err)
	}
	defer rows.Close()

	out := make([]entity.SkorozvonProject, 0, skorozvonProjectPageSize)
	for rows.Next() {
		p, err := scanSkorozvonProject(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *SkorozvonProjectRepo) Get(ctx context.Context, uuid string) (entity.SkorozvonProject, error) {
	p, err := scanSkorozvonProject(r.pool.QueryRow(ctx, `
		SELECT uuid, project_id, name, created_at, last_date_request, last_request
		FROM skorozvon_project
		WHERE uuid = $1
	`, uuid))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.SkorozvonProject{}, fmt.Errorf("%w: skorozvon project %s", domain.ErrNotFound, uuid)
		}
		return entity.SkorozvonProject{}, fmt.Errorf("get skorozvon project: %w", err)
	}
	return p, nil
}

func (r *SkorozvonProjectRepo) Create(ctx context.Context, in entity.SkorozvonProject) (entity.SkorozvonProject, error) {
	p, created, err := r.CreateIfAbsent(ctx, in)
	if err != nil {
		return entity.SkorozvonProject{}, err
	}
	if !created {
		return entity.SkorozvonProject{}, fmt.Errorf("%w: project_id already exists", domain.ErrInvalidInput)
	}
	return p, nil
}

func (r *SkorozvonProjectRepo) CreateIfAbsent(ctx context.Context, in entity.SkorozvonProject) (entity.SkorozvonProject, bool, error) {
	p, err := scanSkorozvonProject(r.pool.QueryRow(ctx, `
		INSERT INTO skorozvon_project (uuid, project_id, name, created_at, last_date_request, last_request)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id) DO NOTHING
		RETURNING uuid, project_id, name, created_at, last_date_request, last_request
	`, in.UUID, in.ProjectID, in.Name, in.CreatedAt, in.LastDateRequest, in.LastRequest))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.SkorozvonProject{}, false, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return entity.SkorozvonProject{}, false, nil
		}
		return entity.SkorozvonProject{}, false, fmt.Errorf("create skorozvon project: %w", err)
	}
	return p, true, nil
}

func (r *SkorozvonProjectRepo) UpdateCheck(ctx context.Context, uuid string, at time.Time, ok bool) (entity.SkorozvonProject, error) {
	p, err := scanSkorozvonProject(r.pool.QueryRow(ctx, `
		UPDATE skorozvon_project
		SET last_date_request = $2, last_request = $3
		WHERE uuid = $1
		RETURNING uuid, project_id, name, created_at, last_date_request, last_request
	`, uuid, at, ok))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.SkorozvonProject{}, fmt.Errorf("%w: skorozvon project %s", domain.ErrNotFound, uuid)
		}
		return entity.SkorozvonProject{}, fmt.Errorf("update skorozvon project check: %w", err)
	}
	return p, nil
}

func scanSkorozvonProject(row rowScanner) (entity.SkorozvonProject, error) {
	var p entity.SkorozvonProject
	if err := row.Scan(&p.UUID, &p.ProjectID, &p.Name, &p.CreatedAt, &p.LastDateRequest, &p.LastRequest); err != nil {
		return entity.SkorozvonProject{}, err
	}
	return p, nil
}
