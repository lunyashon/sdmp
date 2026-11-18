package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.GoogleSheetRepository = (*GoogleSheetRepo)(nil)

type GoogleSheetRepo struct {
	pool *pgxpool.Pool
}

func NewGoogleSheetRepo(pool *pgxpool.Pool) *GoogleSheetRepo {
	return &GoogleSheetRepo{pool: pool}
}

const googleSheetPublicCols = `
	id, name, spreadsheet_id, sheet_name, is_active, test_mode, COALESCE(zarya_results, '{}'),
	COALESCE(credentials->>'client_email', ''),
	COALESCE(credentials->>'project_id', ''),
	COALESCE(credentials->>'private_key', '') <> ''
`

func (r *GoogleSheetRepo) List(ctx context.Context) ([]entity.GoogleSheet, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+googleSheetPublicCols+`
		FROM google_sheet
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list google sheets: %w", err)
	}
	defer rows.Close()

	out := make([]entity.GoogleSheet, 0)
	index := map[int]int{}
	for rows.Next() {
		item, err := scanGoogleSheet(rows)
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

func (r *GoogleSheetRepo) Get(ctx context.Context, id int) (entity.GoogleSheet, error) {
	item, err := scanGoogleSheet(r.pool.QueryRow(ctx, `
		SELECT `+googleSheetPublicCols+`
		FROM google_sheet
		WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.GoogleSheet{}, fmt.Errorf("%w: google sheet %d", domain.ErrNotFound, id)
		}
		return entity.GoogleSheet{}, err
	}
	regions, err := r.loadRegionIDs(ctx, id)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	stops, err := r.loadStopIDs(ctx, id)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	item.RegionIDs = regions[id]
	item.StopSourceIDs = stops[id]
	if item.RegionIDs == nil {
		item.RegionIDs = []int{}
	}
	if item.StopSourceIDs == nil {
		item.StopSourceIDs = []int{}
	}
	return item, nil
}

func (r *GoogleSheetRepo) Create(ctx context.Context, in entity.GoogleSheet, credentials json.RawMessage) (entity.GoogleSheet, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return entity.GoogleSheet{}, fmt.Errorf("begin create google sheet: %w", err)
	}
	defer tx.Rollback(ctx)

	var id int
	err = tx.QueryRow(ctx, `
		INSERT INTO google_sheet (name, spreadsheet_id, sheet_name, credentials, is_active, test_mode, zarya_results, created_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, now(), now())
		RETURNING id
	`, in.Name, in.SpreadsheetID, in.SheetName, []byte(credentials), in.IsActive, in.TestMode, in.ZaryaResults).Scan(&id)
	if err != nil {
		return entity.GoogleSheet{}, fmt.Errorf("create google sheet: %w", err)
	}
	if err := replaceGoogleSheetRelations(ctx, tx, id, in.RegionIDs, in.StopSourceIDs); err != nil {
		return entity.GoogleSheet{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return entity.GoogleSheet{}, fmt.Errorf("commit create google sheet: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *GoogleSheetRepo) Update(ctx context.Context, in entity.GoogleSheet, credentials json.RawMessage) (entity.GoogleSheet, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return entity.GoogleSheet{}, fmt.Errorf("begin update google sheet: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE google_sheet
		SET name = $2, spreadsheet_id = $3, sheet_name = $4, is_active = $5, test_mode = $6, zarya_results = $7, updated_at = now()
		WHERE id = $1
	`
	args := []any{in.ID, in.Name, in.SpreadsheetID, in.SheetName, in.IsActive, in.TestMode, in.ZaryaResults}
	if len(credentials) > 0 {
		query = `
			UPDATE google_sheet
			SET name = $2, spreadsheet_id = $3, sheet_name = $4, is_active = $5, test_mode = $6, zarya_results = $7, credentials = $8::jsonb, updated_at = now()
			WHERE id = $1
		`
		args = append(args, []byte(credentials))
	}
	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return entity.GoogleSheet{}, fmt.Errorf("update google sheet: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return entity.GoogleSheet{}, fmt.Errorf("%w: google sheet %d", domain.ErrNotFound, in.ID)
	}
	if err := replaceGoogleSheetRelations(ctx, tx, in.ID, in.RegionIDs, in.StopSourceIDs); err != nil {
		return entity.GoogleSheet{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return entity.GoogleSheet{}, fmt.Errorf("commit update google sheet: %w", err)
	}
	return r.Get(ctx, in.ID)
}

func (r *GoogleSheetRepo) ListRoutable(ctx context.Context, zarya string, regionID *int, sourceID int) ([]entity.GoogleSheetTarget, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT g.id, g.name, g.spreadsheet_id, g.sheet_name, g.credentials
		FROM google_sheet g
		WHERE g.is_active = true
		  AND $1 = ANY(g.zarya_results)
		  AND (
		    NOT EXISTS (SELECT 1 FROM google_sheet_region r WHERE r.sheet_id = g.id)
		    OR (
		      $2::int IS NOT NULL
		      AND EXISTS (SELECT 1 FROM google_sheet_region r WHERE r.sheet_id = g.id AND r.region_id = $2)
		    )
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM google_sheet_stop_source s
		    WHERE s.sheet_id = g.id
		      AND s.source_id = $3
		  )
		ORDER BY g.id
	`, zarya, regionID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list routable google sheets: %w", err)
	}
	defer rows.Close()

	out := make([]entity.GoogleSheetTarget, 0)
	for rows.Next() {
		var item entity.GoogleSheetTarget
		if err := rows.Scan(&item.ID, &item.Name, &item.SpreadsheetID, &item.SheetName, &item.Credentials); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *GoogleSheetRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM google_sheet WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete google sheet: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: google sheet %d", domain.ErrNotFound, id)
	}
	return nil
}

func replaceGoogleSheetRelations(ctx context.Context, tx pgx.Tx, sheetID int, regionIDs, stopIDs []int) error {
	if _, err := tx.Exec(ctx, `DELETE FROM google_sheet_region WHERE sheet_id = $1`, sheetID); err != nil {
		return fmt.Errorf("clear google sheet regions: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM google_sheet_stop_source WHERE sheet_id = $1`, sheetID); err != nil {
		return fmt.Errorf("clear google sheet stops: %w", err)
	}
	if len(regionIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO google_sheet_region (sheet_id, region_id)
			SELECT $1, unnest($2::int[])
		`, sheetID, regionIDs); err != nil {
			return wrapGoogleSheetFK(err, "set google sheet regions")
		}
	}
	if len(stopIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO google_sheet_stop_source (sheet_id, source_id)
			SELECT $1, unnest($2::int[])
		`, sheetID, stopIDs); err != nil {
			return wrapGoogleSheetFK(err, "set google sheet stops")
		}
	}
	return nil
}

func wrapGoogleSheetFK(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return fmt.Errorf("%w: invalid region or source", domain.ErrInvalidInput)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (r *GoogleSheetRepo) loadRegionIDs(ctx context.Context, only ...int) (map[int][]int, error) {
	q := `SELECT sheet_id, region_id FROM google_sheet_region`
	args := []any{}
	if len(only) == 1 {
		q += ` WHERE sheet_id = $1`
		args = append(args, only[0])
	}
	q += ` ORDER BY region_id`
	return loadIntGroups(ctx, r.pool, q, args...)
}

func (r *GoogleSheetRepo) loadStopIDs(ctx context.Context, only ...int) (map[int][]int, error) {
	q := `SELECT sheet_id, source_id FROM google_sheet_stop_source`
	args := []any{}
	if len(only) == 1 {
		q += ` WHERE sheet_id = $1`
		args = append(args, only[0])
	}
	q += ` ORDER BY source_id`
	return loadIntGroups(ctx, r.pool, q, args...)
}

type googleSheetScanner interface {
	Scan(dest ...any) error
}

func scanGoogleSheet(row googleSheetScanner) (entity.GoogleSheet, error) {
	var item entity.GoogleSheet
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.SpreadsheetID,
		&item.SheetName,
		&item.IsActive,
		&item.TestMode,
		&item.ZaryaResults,
		&item.ClientEmail,
		&item.ProjectID,
		&item.HasKey,
	); err != nil {
		return entity.GoogleSheet{}, err
	}
	if item.ZaryaResults == nil {
		item.ZaryaResults = []string{}
	}
	item.RegionIDs = []int{}
	item.StopSourceIDs = []int{}
	return item, nil
}
