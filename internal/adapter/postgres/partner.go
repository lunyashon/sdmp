package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.PartnerRepository = (*PartnerRepo)(nil)

type PartnerRepo struct {
	pool *pgxpool.Pool
}

func NewPartnerRepo(pool *pgxpool.Pool) *PartnerRepo {
	return &PartnerRepo{pool: pool}
}

func (r *PartnerRepo) List(ctx context.Context) ([]entity.Partner, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, webhook_url, is_active, COALESCE(zarya_results, '{}'),
		       COALESCE(body, '{}'), COALESCE(NULLIF(content_type, ''), 'application/x-www-form-urlencoded'),
		       last_date_request, last_request, last_http_status, COALESCE(last_response, '')
		FROM upk
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list partners: %w", err)
	}
	defer rows.Close()

	out := make([]entity.Partner, 0)
	index := map[int]int{}
	for rows.Next() {
		p, err := scanPartnerRow(rows)
		if err != nil {
			return nil, err
		}
		index[p.ID] = len(out)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	var regions map[int][]int
	var stops map[int][]int
	errg, ctxg := errgroup.WithContext(ctx)
	errg.Go(func() error {
		var err error
		regions, err = r.loadRegionIDs(ctxg)
		return err
	})
	errg.Go(func() error {
		var err error
		stops, err = r.loadStopIDs(ctxg)
		return err
	})
	if err := errg.Wait(); err != nil {
		return nil, err
	}
	for id, ids := range regions {
		if i, ok := index[id]; ok {
			out[i].RegionIDs = ids
		}
	}
	for id, ids := range stops {
		if i, ok := index[id]; ok {
			out[i].StopSourceIDs = ids
		}
	}
	return out, nil
}

func (r *PartnerRepo) Get(ctx context.Context, id int) (entity.Partner, error) {
	p, err := scanPartnerRow(r.pool.QueryRow(ctx, `
		SELECT id, name, webhook_url, is_active, COALESCE(zarya_results, '{}'),
		       COALESCE(body, '{}'), COALESCE(NULLIF(content_type, ''), 'application/x-www-form-urlencoded'),
		       last_date_request, last_request, last_http_status, COALESCE(last_response, '')
		FROM upk
		WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Partner{}, fmt.Errorf("%w: partner id %d", domain.ErrNotFound, id)
		}
		return entity.Partner{}, fmt.Errorf("get partner: %w", err)
	}

	var regions map[int][]int
	var stops map[int][]int
	errg, ctxg := errgroup.WithContext(ctx)
	errg.Go(func() error {
		var err error
		regions, err = r.loadRegionIDs(ctxg, id)
		return err
	})
	errg.Go(func() error {
		var err error
		stops, err = r.loadStopIDs(ctxg, id)
		return err
	})
	if err := errg.Wait(); err != nil {
		return entity.Partner{}, err
	}
	p.RegionIDs = regions[id]
	p.StopSourceIDs = stops[id]
	if p.RegionIDs == nil {
		p.RegionIDs = []int{}
	}
	if p.StopSourceIDs == nil {
		p.StopSourceIDs = []int{}
	}
	return p, nil
}

func (r *PartnerRepo) ListRoutable(ctx context.Context, zarya string, regionID *int, sourceID int) ([]entity.Partner, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.name, u.webhook_url, u.is_active, COALESCE(u.zarya_results, '{}'),
		       COALESCE(u.body, '{}'), COALESCE(NULLIF(u.content_type, ''), 'application/x-www-form-urlencoded'),
		       u.last_date_request, u.last_request, u.last_http_status, COALESCE(u.last_response, '')
		FROM upk u
		WHERE u.is_active = true
		  AND $1 = ANY(u.zarya_results)
		  AND (
		    NOT EXISTS (SELECT 1 FROM upk_region r WHERE r.upk_id = u.id)
		    OR (
		      $2::int IS NOT NULL
		      AND EXISTS (SELECT 1 FROM upk_region r WHERE r.upk_id = u.id AND r.region_id = $2)
		    )
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM source_upk s
		    WHERE s.upk_id = u.id
		      AND s.allowed = false
		      AND s.source_id = $3
		  )
		ORDER BY u.id
	`, zarya, regionID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list routable partners: %w", err)
	}
	defer rows.Close()

	out := make([]entity.Partner, 0)
	for rows.Next() {
		p, err := scanPartnerRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PartnerRepo) Create(ctx context.Context, in entity.Partner) (entity.Partner, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return entity.Partner{}, fmt.Errorf("begin create partner: %w", err)
	}
	defer tx.Rollback(ctx)

	body, err := marshalPartnerBody(in.Body)
	if err != nil {
		return entity.Partner{}, err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO upk (name, webhook_url, is_active, zarya_results, body, content_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, in.Name, in.WebhookURL, in.IsActive, in.ZaryaResults, body, partnerContentType(in.ContentType)).Scan(&in.ID); err != nil {
		return entity.Partner{}, fmt.Errorf("create partner: %w", err)
	}
	if err := replacePartnerRelations(ctx, tx, in.ID, in.RegionIDs, in.StopSourceIDs); err != nil {
		return entity.Partner{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return entity.Partner{}, fmt.Errorf("commit create partner: %w", err)
	}
	return r.Get(ctx, in.ID)
}

func (r *PartnerRepo) Update(ctx context.Context, in entity.Partner) (entity.Partner, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return entity.Partner{}, fmt.Errorf("begin update partner: %w", err)
	}
	defer tx.Rollback(ctx)

	body, err := marshalPartnerBody(in.Body)
	if err != nil {
		return entity.Partner{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE upk
		SET name = $2, webhook_url = $3, is_active = $4, zarya_results = $5, body = $6, content_type = $7
		WHERE id = $1
	`, in.ID, in.Name, in.WebhookURL, in.IsActive, in.ZaryaResults, body, partnerContentType(in.ContentType))
	if err != nil {
		return entity.Partner{}, fmt.Errorf("update partner: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return entity.Partner{}, fmt.Errorf("%w: partner id %d", domain.ErrNotFound, in.ID)
	}
	if err := replacePartnerRelations(ctx, tx, in.ID, in.RegionIDs, in.StopSourceIDs); err != nil {
		return entity.Partner{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return entity.Partner{}, fmt.Errorf("commit update partner: %w", err)
	}
	return r.Get(ctx, in.ID)
}

func (r *PartnerRepo) UpdateCheck(ctx context.Context, id int, at time.Time, ok bool, status int, response string) (entity.Partner, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE upk
		SET last_date_request = $2,
		    last_request = $3,
		    last_http_status = NULLIF($4, 0),
		    last_response = NULLIF($5, '')
		WHERE id = $1
	`, id, at, ok, status, response)
	if err != nil {
		return entity.Partner{}, fmt.Errorf("update partner check: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return entity.Partner{}, fmt.Errorf("%w: partner id %d", domain.ErrNotFound, id)
	}
	return r.Get(ctx, id)
}

func replacePartnerRelations(ctx context.Context, tx pgx.Tx, upkID int, regionIDs, stopIDs []int) error {
	if _, err := tx.Exec(ctx, `DELETE FROM upk_region WHERE upk_id = $1`, upkID); err != nil {
		return fmt.Errorf("clear partner regions: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM source_upk WHERE upk_id = $1`, upkID); err != nil {
		return fmt.Errorf("clear partner stops: %w", err)
	}
	if len(regionIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO upk_region (upk_id, region_id)
			SELECT $1, unnest($2::int[])
		`, upkID, regionIDs); err != nil {
			return wrapPartnerFK(err, "set partner regions")
		}
	}
	if len(stopIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO source_upk (source_id, upk_id, allowed)
			SELECT src, $1, false
			FROM unnest($2::int[]) AS src
		`, upkID, stopIDs); err != nil {
			return wrapPartnerFK(err, "set partner stops")
		}
	}
	return nil
}

func wrapPartnerFK(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return fmt.Errorf("%w: invalid region or source", domain.ErrInvalidInput)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (r *PartnerRepo) loadRegionIDs(ctx context.Context, only ...int) (map[int][]int, error) {
	q := `SELECT upk_id, region_id FROM upk_region`
	args := []any{}
	if len(only) == 1 {
		q += ` WHERE upk_id = $1`
		args = append(args, only[0])
	}
	q += ` ORDER BY region_id`
	return loadIntGroups(ctx, r.pool, q, args...)
}

func (r *PartnerRepo) loadStopIDs(ctx context.Context, only ...int) (map[int][]int, error) {
	q := `SELECT upk_id, source_id FROM source_upk WHERE allowed = false`
	args := []any{}
	if len(only) == 1 {
		q += ` AND upk_id = $1`
		args = append(args, only[0])
	}
	q += ` ORDER BY source_id`
	return loadIntGroups(ctx, r.pool, q, args...)
}

func loadIntGroups(ctx context.Context, pool *pgxpool.Pool, q string, args ...any) (map[int][]int, error) {
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("load partner refs: %w", err)
	}
	defer rows.Close()
	out := map[int][]int{}
	for rows.Next() {
		var key, val int
		if err := rows.Scan(&key, &val); err != nil {
			return nil, err
		}
		out[key] = append(out[key], val)
	}
	return out, rows.Err()
}

func scanPartnerRow(row rowScanner) (entity.Partner, error) {
	var p entity.Partner
	var body []byte
	if err := row.Scan(
		&p.ID,
		&p.Name,
		&p.WebhookURL,
		&p.IsActive,
		&p.ZaryaResults,
		&body,
		&p.ContentType,
		&p.LastDateRequest,
		&p.LastRequest,
		&p.LastHTTPStatus,
		&p.LastResponse,
	); err != nil {
		return entity.Partner{}, err
	}
	if p.ZaryaResults == nil {
		p.ZaryaResults = []string{}
	}
	p.Body = unmarshalPartnerBody(body)
	if p.ContentType == "" {
		p.ContentType = entity.PartnerContentTypeForm
	}
	p.RegionIDs = []int{}
	p.StopSourceIDs = []int{}
	return p, nil
}

func marshalPartnerBody(in map[string]string) ([]byte, error) {
	if in == nil {
		in = map[string]string{}
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("marshal partner body: %w", err)
	}
	return raw, nil
}

func unmarshalPartnerBody(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]string{}
	}
	if out == nil {
		return map[string]string{}
	}
	return out
}

func partnerContentType(v string) string {
	if v == "" {
		return entity.PartnerContentTypeForm
	}
	return v
}
