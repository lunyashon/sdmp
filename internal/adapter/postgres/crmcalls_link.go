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

var _ port.CrmcallsLinkRepository = (*CrmcallsLinkRepo)(nil)

type CrmcallsLinkRepo struct {
	pool *pgxpool.Pool
}

func NewCrmcallsLinkRepo(pool *pgxpool.Pool) *CrmcallsLinkRepo {
	return &CrmcallsLinkRepo{pool: pool}
}

func (r *CrmcallsLinkRepo) List(ctx context.Context) ([]entity.CrmcallsLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, "type", project_id
		FROM crmcalls_link
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list crmcalls links: %w", err)
	}
	defer rows.Close()

	out := make([]entity.CrmcallsLink, 0)
	index := map[int]int{}
	for rows.Next() {
		item, err := scanCrmcallsLink(rows)
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
		sources, err = loadLinkInts(ctxg, r.pool, `SELECT link_id, source_id FROM crmcalls_link_source`)
		return err
	})
	errg.Go(func() error {
		var err error
		regions, err = loadLinkInts(ctxg, r.pool, `SELECT link_id, region_id FROM crmcalls_link_region`)
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

func (r *CrmcallsLinkRepo) ListRoutable(ctx context.Context, kind string, childSourceID int, regionID *int) ([]entity.CrmcallsLink, error) {
	if childSourceID <= 0 {
		return []entity.CrmcallsLink{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT l.id, l.name, l."type", l.project_id
		FROM crmcalls_link l
		WHERE l."type" = $1
		  AND EXISTS (
		    SELECT 1 FROM crmcalls_link_source s
		    WHERE s.link_id = l.id AND s.source_id = $2
		  )
		  AND (
		    NOT EXISTS (SELECT 1 FROM crmcalls_link_region r WHERE r.link_id = l.id)
		    OR (
		      $3::int IS NOT NULL
		      AND EXISTS (SELECT 1 FROM crmcalls_link_region r WHERE r.link_id = l.id AND r.region_id = $3)
		    )
		  )
		ORDER BY l.id
	`, kind, childSourceID, regionID)
	if err != nil {
		return nil, fmt.Errorf("list routable crmcalls links: %w", err)
	}
	defer rows.Close()
	out := make([]entity.CrmcallsLink, 0)
	for rows.Next() {
		item, err := scanCrmcallsLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *CrmcallsLinkRepo) Get(ctx context.Context, id int) (entity.CrmcallsLink, error) {
	item, err := scanCrmcallsLink(r.pool.QueryRow(ctx, `
		SELECT id, name, "type", project_id FROM crmcalls_link WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.CrmcallsLink{}, fmt.Errorf("%w: crmcalls link %d", domain.ErrNotFound, id)
		}
		return entity.CrmcallsLink{}, err
	}
	sources, _ := loadLinkInts(ctx, r.pool, `SELECT link_id, source_id FROM crmcalls_link_source`)
	regions, _ := loadLinkInts(ctx, r.pool, `SELECT link_id, region_id FROM crmcalls_link_region`)
	item.ChildSourceIDs = sources[item.ID]
	item.RegionIDs = regions[item.ID]
	return item, nil
}

func (r *CrmcallsLinkRepo) Create(ctx context.Context, in entity.CrmcallsLink) (entity.CrmcallsLink, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO crmcalls_link (name, "type", project_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, in.Name, in.Type, in.ProjectID).Scan(&in.ID)
	if err != nil {
		return entity.CrmcallsLink{}, fmt.Errorf("create crmcalls link: %w", err)
	}
	if err := r.replaceMembers(ctx, in.ID, in.ChildSourceIDs, in.RegionIDs); err != nil {
		return entity.CrmcallsLink{}, err
	}
	return r.Get(ctx, in.ID)
}

func (r *CrmcallsLinkRepo) Update(ctx context.Context, in entity.CrmcallsLink) (entity.CrmcallsLink, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE crmcalls_link SET name = $2, "type" = $3, project_id = $4 WHERE id = $1
	`, in.ID, in.Name, in.Type, in.ProjectID)
	if err != nil {
		return entity.CrmcallsLink{}, fmt.Errorf("update crmcalls link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return entity.CrmcallsLink{}, fmt.Errorf("%w: crmcalls link %d", domain.ErrNotFound, in.ID)
	}
	if err := r.replaceMembers(ctx, in.ID, in.ChildSourceIDs, in.RegionIDs); err != nil {
		return entity.CrmcallsLink{}, err
	}
	return r.Get(ctx, in.ID)
}

func (r *CrmcallsLinkRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM crmcalls_link WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete crmcalls link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: crmcalls link %d", domain.ErrNotFound, id)
	}
	return nil
}

func (r *CrmcallsLinkRepo) replaceMembers(ctx context.Context, linkID int, sources, regions []int) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM crmcalls_link_source WHERE link_id = $1`, linkID); err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM crmcalls_link_region WHERE link_id = $1`, linkID); err != nil {
		return err
	}
	for _, id := range sources {
		if _, err := r.pool.Exec(ctx, `INSERT INTO crmcalls_link_source (link_id, source_id) VALUES ($1, $2)`, linkID, id); err != nil {
			return err
		}
	}
	for _, id := range regions {
		if _, err := r.pool.Exec(ctx, `INSERT INTO crmcalls_link_region (link_id, region_id) VALUES ($1, $2)`, linkID, id); err != nil {
			return err
		}
	}
	return nil
}

type crmcallsLinkScanner interface {
	Scan(dest ...any) error
}

func scanCrmcallsLink(row crmcallsLinkScanner) (entity.CrmcallsLink, error) {
	var item entity.CrmcallsLink
	if err := row.Scan(&item.ID, &item.Name, &item.Type, &item.ProjectID); err != nil {
		return entity.CrmcallsLink{}, err
	}
	item.Provider = "crmcalls"
	return item, nil
}
