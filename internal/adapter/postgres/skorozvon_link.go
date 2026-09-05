package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.SkorozvonLinkRepository = (*SkorozvonLinkRepo)(nil)

type SkorozvonLinkRepo struct {
	pool *pgxpool.Pool
}

func NewSkorozvonLinkRepo(pool *pgxpool.Pool) *SkorozvonLinkRepo {
	return &SkorozvonLinkRepo{pool: pool}
}

func (r *SkorozvonLinkRepo) List(ctx context.Context) ([]entity.SkorozvonLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, "type", project_id
		FROM skorozvon_link
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list skorozvon links: %w", err)
	}
	defer rows.Close()

	out := make([]entity.SkorozvonLink, 0)
	index := map[int]int{}
	for rows.Next() {
		item, err := scanSkorozvonLink(rows)
		if err != nil {
			return nil, err
		}
		index[item.ID] = len(out)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	var sources map[int][]int
	var regions map[int][]int
	errg, ctxg := errgroup.WithContext(ctx)
	errg.Go(func() error {
		var err error
		sources, err = r.loadSourceIDs(ctxg)
		return err
	})
	errg.Go(func() error {
		var err error
		regions, err = r.loadRegionIDs(ctxg)
		return err
	})
	if err := errg.Wait(); err != nil {
		return nil, err
	}
	for id, ids := range sources {
		if i, ok := index[id]; ok {
			out[i].ChildSourceIDs = ids
		}
	}
	for id, ids := range regions {
		if i, ok := index[id]; ok {
			out[i].RegionIDs = ids
		}
	}
	return out, nil
}

func (r *SkorozvonLinkRepo) ListRoutable(ctx context.Context, kind string, childSourceID int, regionID *int) ([]entity.SkorozvonLink, error) {
	if childSourceID <= 0 {
		return []entity.SkorozvonLink{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT l.id, l.name, l."type", l.project_id
		FROM skorozvon_link l
		WHERE l."type" = $1
		  AND EXISTS (
		    SELECT 1 FROM skorozvon_link_source s
		    WHERE s.link_id = l.id AND s.source_id = $2
		  )
		  AND (
		    NOT EXISTS (SELECT 1 FROM skorozvon_link_region r WHERE r.link_id = l.id)
		    OR (
		      $3::int IS NOT NULL
		      AND EXISTS (SELECT 1 FROM skorozvon_link_region r WHERE r.link_id = l.id AND r.region_id = $3)
		    )
		  )
		ORDER BY l.id
	`, kind, childSourceID, regionID)
	if err != nil {
		return nil, fmt.Errorf("list routable skorozvon links: %w", err)
	}
	defer rows.Close()
	out := make([]entity.SkorozvonLink, 0)
	for rows.Next() {
		item, err := scanSkorozvonLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SkorozvonLinkRepo) Get(ctx context.Context, id int) (entity.SkorozvonLink, error) {
	item, err := scanSkorozvonLink(r.pool.QueryRow(ctx, `
		SELECT id, name, "type", project_id FROM skorozvon_link WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.SkorozvonLink{}, fmt.Errorf("%w: skorozvon link %d", domain.ErrNotFound, id)
		}
		return entity.SkorozvonLink{}, err
	}
	sources, _ := r.loadSourceIDs(ctx)
	regions, _ := r.loadRegionIDs(ctx)
	item.ChildSourceIDs = sources[item.ID]
	item.RegionIDs = regions[item.ID]
	return item, nil
}

func (r *SkorozvonLinkRepo) Create(ctx context.Context, in entity.SkorozvonLink) (entity.SkorozvonLink, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO skorozvon_link (name, "type", project_id, created_at)
		VALUES ($1, $2, $3, now())
		RETURNING id
	`, in.Name, in.Type, in.ProjectID).Scan(&in.ID)
	if err != nil {
		return entity.SkorozvonLink{}, fmt.Errorf("create skorozvon link: %w", err)
	}
	if err := r.replaceMembers(ctx, in.ID, in.ChildSourceIDs, in.RegionIDs); err != nil {
		return entity.SkorozvonLink{}, err
	}
	return r.Get(ctx, in.ID)
}

func (r *SkorozvonLinkRepo) Update(ctx context.Context, in entity.SkorozvonLink) (entity.SkorozvonLink, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE skorozvon_link SET name = $2, "type" = $3, project_id = $4 WHERE id = $1
	`, in.ID, in.Name, in.Type, in.ProjectID)
	if err != nil {
		return entity.SkorozvonLink{}, fmt.Errorf("update skorozvon link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return entity.SkorozvonLink{}, fmt.Errorf("%w: skorozvon link %d", domain.ErrNotFound, in.ID)
	}
	if err := r.replaceMembers(ctx, in.ID, in.ChildSourceIDs, in.RegionIDs); err != nil {
		return entity.SkorozvonLink{}, err
	}
	return r.Get(ctx, in.ID)
}

func (r *SkorozvonLinkRepo) replaceMembers(ctx context.Context, linkID int, sources, regions []int) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM skorozvon_link_source WHERE link_id = $1`, linkID); err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM skorozvon_link_region WHERE link_id = $1`, linkID); err != nil {
		return err
	}
	for _, id := range sources {
		if _, err := r.pool.Exec(ctx, `INSERT INTO skorozvon_link_source (link_id, source_id) VALUES ($1, $2)`, linkID, id); err != nil {
			return err
		}
	}
	for _, id := range regions {
		if _, err := r.pool.Exec(ctx, `INSERT INTO skorozvon_link_region (link_id, region_id) VALUES ($1, $2)`, linkID, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *SkorozvonLinkRepo) loadSourceIDs(ctx context.Context) (map[int][]int, error) {
	return loadLinkInts(ctx, r.pool, `SELECT link_id, source_id FROM skorozvon_link_source`)
}

func (r *SkorozvonLinkRepo) loadRegionIDs(ctx context.Context) (map[int][]int, error) {
	return loadLinkInts(ctx, r.pool, `SELECT link_id, region_id FROM skorozvon_link_region`)
}

func loadLinkInts(ctx context.Context, pool *pgxpool.Pool, q string) (map[int][]int, error) {
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int][]int{}
	for rows.Next() {
		var linkID, id int
		if err := rows.Scan(&linkID, &id); err != nil {
			return nil, err
		}
		out[linkID] = append(out[linkID], id)
	}
	return out, rows.Err()
}

type linkScanner interface {
	Scan(dest ...any) error
}

func scanSkorozvonLink(row linkScanner) (entity.SkorozvonLink, error) {
	var item entity.SkorozvonLink
	if err := row.Scan(&item.ID, &item.Name, &item.Type, &item.ProjectID); err != nil {
		return entity.SkorozvonLink{}, err
	}
	return item, nil
}
