package postgres

import (
	"context"
	"errors"
	"fmt"
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
	p, err := scanSkorozvonProject(r.pool.QueryRow(ctx, `
		INSERT INTO skorozvon_project (uuid, project_id, name, created_at, last_date_request, last_request)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING uuid, project_id, name, created_at, last_date_request, last_request
	`, in.UUID, in.ProjectID, in.Name, in.CreatedAt, in.LastDateRequest, in.LastRequest))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return entity.SkorozvonProject{}, fmt.Errorf("%w: project_id already exists", domain.ErrInvalidInput)
		}
		return entity.SkorozvonProject{}, fmt.Errorf("create skorozvon project: %w", err)
	}
	return p, nil
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
