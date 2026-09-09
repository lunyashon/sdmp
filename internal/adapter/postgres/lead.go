package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/lib/id"
	"github.com/lunyashon/sdmp/internal/lib/phone"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.LeadRepository = (*LeadRepo)(nil)

type LeadRepo struct {
	pool *pgxpool.Pool
}

func NewLeadRepo(pool *pgxpool.Pool) *LeadRepo {
	return &LeadRepo{pool: pool}
}

func (r *LeadRepo) InsertAccepted(ctx context.Context, row entity.LeadRow, outbox entity.OutboxMessage) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin accept txn: %w", err)
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO lead_row (uuid, phone, source_id, web_id, status_id, s3_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, row.UUID, row.Phone, row.SourceID, nilIfEmpty(row.WebID), row.StatusID, row.S3Key, row.CreatedAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert lead_row: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox (lead_row_id, source_id, payload, created_at)
		VALUES ($1, $2, $3, $4)
	`, id, outbox.SourceID, outbox.Payload, outbox.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert outbox: %w", err)
	}

	if err := insertLeadEvent(ctx, tx, id, entity.EventAccepted, entity.EventSourceAPI, ""); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit accept txn: %w", err)
	}
	return id, nil
}

func (r *LeadRepo) UpdateEnrichment(ctx context.Context, leadID int64, in entity.LeadEnrichment) error {
	_, err := r.pool.Exec(ctx, `
		WITH upd AS (
			UPDATE lead_row
			SET region_id = $2,
			    region_source = $3,
			    operator_id = $4,
			    operator_raw = NULLIF($5, ''),
			    operator_source = $6,
			    child_source_id = $7,
			    cold_label = NULLIF($8, ''),
			    base_label = NULLIF($9, ''),
			    tag = NULLIF($10, ''),
			    status_id = $11,
			    updated_at = now()
			WHERE id = $1
			RETURNING id
		)
		INSERT INTO event (lead_row_id, event_type_id, occurred_at, source)
		SELECT id, $12, now(), $13 FROM upd
	`, leadID, in.RegionID, in.RegionSource, in.OperatorID, in.OperatorRaw, in.OperatorSource, in.ChildSourceID, in.ColdLabel, in.BaseLabel, in.Tag, in.StatusID, entity.EventEnriched, entity.EventSourceWorker)
	if err != nil {
		return fmt.Errorf("update lead enrichment: %w", err)
	}
	return nil
}

func (r *LeadRepo) BindBitrix(ctx context.Context, leadUUID, bitrixLeadID string) error {
	_, err := r.pool.Exec(ctx, `
		WITH upd AS (
			UPDATE lead_row
			SET bitrix_lead_id = NULLIF($2, ''),
			    status_id = $3,
			    updated_at = now()
			WHERE uuid = $1
			RETURNING id
		),
		ev AS (
			INSERT INTO event (lead_row_id, event_type_id, occurred_at, source, details)
			SELECT id, $4, now(), $5, $2 FROM upd
		)
		INSERT INTO dispatch (lead_row_id, channel_id, external_id, sent_at, sent_ok, attempts)
		SELECT id, $6, NULLIF($2, ''), now(), true, 1 FROM upd
	`, leadUUID, bitrixLeadID, entity.StatusInB24, entity.EventInB24, entity.EventSourceWorker, entity.ChannelBitrix)
	if err != nil {
		return fmt.Errorf("bind bitrix: %w", err)
	}
	return nil
}

func (r *LeadRepo) BindSkorozvon(ctx context.Context, leadID int64, skorozvonLeadID, projectID string) error {
	_, err := r.pool.Exec(ctx, `
		WITH upd AS (
			UPDATE lead_row
			SET skorozvon_lead_id = NULLIF($2, ''),
			    project_num = NULLIF($3, ''),
			    status_id = $4,
			    updated_at = now()
			WHERE id = $1
			RETURNING id
		)
		INSERT INTO event (lead_row_id, event_type_id, occurred_at, source, details)
		SELECT id, $5, now(), $6, NULLIF($2, '') FROM upd
	`, leadID, skorozvonLeadID, projectID, entity.StatusInSK, entity.EventInSK, entity.EventSourceWorker)
	if err != nil {
		return fmt.Errorf("bind skorozvon: %w", err)
	}
	return nil
}

func (r *LeadRepo) ApplyCallResult(ctx context.Context, skorozvonLeadID string, in entity.LeadCallResult) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE lead_row
		SET call_id = NULLIF($2, ''),
		    call_url = NULLIF($3, ''),
		    call_last_at = $4,
		    call_result = $5,
		    call_result_name = COALESCE(NULLIF($7, ''), call_result_name),
		    project_num = COALESCE(NULLIF($6, ''), project_num),
		    updated_at = now()
		WHERE skorozvon_lead_id = $1
	`, skorozvonLeadID, in.CallID, in.CallURL, in.CallLastAt, in.CallResult, in.ProjectNum, in.CallResultName)
	if err != nil {
		return fmt.Errorf("apply call result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *LeadRepo) UpdateSkorozvonGeo(ctx context.Context, skorozvonLeadID string, in entity.LeadFromSkorozvon) error {
	city := strings.TrimSpace(in.City)
	if city == "" && in.RegionID == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE lead_row
		SET city = COALESCE(NULLIF(btrim(city), ''), NULLIF($2, '')),
		    region_id = COALESCE(region_id, $3),
		    region_source = CASE
		        WHEN region_id IS NULL AND $3 IS NOT NULL THEN $4
		        ELSE region_source
		    END,
		    updated_at = now()
		WHERE skorozvon_lead_id = $1
	`, skorozvonLeadID, city, in.RegionID, in.RegionSource)
	if err != nil {
		return fmt.Errorf("update skorozvon geo: %w", err)
	}
	return nil
}

func (r *LeadRepo) ReplaceSkorozvonAnketaFields(ctx context.Context, skorozvonLeadID string, fields []entity.SkorozvonAnketaField) error {
	if skorozvonLeadID == "" {
		return fmt.Errorf("replace skorozvon anketa fields: empty skorozvon_lead_id")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("replace skorozvon anketa fields begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var leadRowID int64
	err = tx.QueryRow(ctx, `SELECT id FROM lead_row WHERE skorozvon_lead_id = $1 LIMIT 1`, skorozvonLeadID).Scan(&leadRowID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("replace skorozvon anketa fields: lead not found")
		}
		return fmt.Errorf("replace skorozvon anketa fields lookup: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skorozvon_anketa_fields WHERE skorozvon_lead_id = $1`, skorozvonLeadID); err != nil {
		return fmt.Errorf("delete skorozvon anketa fields: %w", err)
	}
	for _, f := range fields {
		if f.FieldID <= 0 || f.EnumID <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO skorozvon_anketa_fields (lead_row_id, skorozvon_lead_id, field_id, enum_id)
			VALUES ($1, $2, $3, $4)
		`, leadRowID, skorozvonLeadID, f.FieldID, f.EnumID); err != nil {
			return fmt.Errorf("insert skorozvon anketa field: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("replace skorozvon anketa fields commit: %w", err)
	}
	return nil
}

func (r *LeadRepo) EnqueueBx24(ctx context.Context, skorozvonLeadID string) error {
	skorozvonLeadID = strings.TrimSpace(skorozvonLeadID)
	if skorozvonLeadID == "" {
		return fmt.Errorf("enqueue bx24: empty skorozvon_lead_id")
	}
	outboxID, err := id.NewUUID()
	if err != nil {
		return fmt.Errorf("outbox_bx24 id: %w", err)
	}
	var leadUUID string
	err = r.pool.QueryRow(ctx, `
		WITH lead AS (
			SELECT id, uuid
			FROM lead_row
			WHERE skorozvon_lead_id = $1
			  AND COALESCE(bitrix_lead_id, '') = ''
		),
		upd AS (
			UPDATE lead_row lr
			SET status_id = $2,
			    updated_at = now()
			FROM lead
			WHERE lr.id = lead.id
			RETURNING lr.id, lr.uuid
		),
		queued AS (
			INSERT INTO outbox_bx24 (uuid, uuid_lead, locked_until)
			SELECT $3, uuid, NULL
			FROM upd
			ON CONFLICT (uuid_lead) DO NOTHING
			RETURNING uuid_lead
		),
		ev AS (
			INSERT INTO event (lead_row_id, event_type_id, occurred_at, source)
			SELECT id, $4, now(), $5
			FROM upd
			JOIN queued ON queued.uuid_lead = upd.uuid
		)
		SELECT uuid FROM upd
	`, skorozvonLeadID, entity.StatusFromSK, outboxID, entity.EventFromSK, entity.EventSourceSK).Scan(&leadUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("enqueue bx24: %w", err)
	}
	return nil
}

func (r *LeadRepo) ApplyFormResponse(ctx context.Context, skorozvonLeadID string, in entity.LeadFormResponse) error {
	answers, err := json.Marshal(in.Answers)
	if err != nil {
		return fmt.Errorf("marshal anketa answers: %w", err)
	}
	if in.Answers == nil {
		answers = []byte("[]")
	}

	var leadUUID string
	err = r.pool.QueryRow(ctx, `
		WITH lead AS (
			SELECT id, uuid, via_us FROM lead_row WHERE skorozvon_lead_id = $1
		),
		ins AS (
			INSERT INTO skorozvon_anketa (
				lead_row_id, skorozvon_lead_id, call_id, user_id, form_id, scenario_id, form_name, answers, form_updated_at
			)
			SELECT lead.id, $1, NULLIF($2, ''), $3, $4, $5, NULLIF($6, ''), $7, $8
			FROM lead
			RETURNING id, lead_row_id
		),
		updated AS (
			UPDATE lead_row lr
			SET skorozvon_anketa_id = ins.id,
			    status_id = $9,
			    updated_at = now()
			FROM ins
			WHERE lr.id = ins.lead_row_id
			RETURNING lr.id, lr.uuid, lr.via_us
		),
		ev AS (
			INSERT INTO event (lead_row_id, event_type_id, occurred_at, source)
			SELECT id, $10, now(), $11
			FROM updated
			WHERE via_us
		)
		SELECT uuid FROM updated
	`, skorozvonLeadID, in.CallID, in.UserID, in.FormID, in.ScenarioID, in.FormName, string(answers), in.UpdatedAt, entity.StatusFromSK, entity.EventFromSK, entity.EventSourceSK).Scan(&leadUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("apply form response: %w", err)
	}
	return nil
}

func (r *LeadRepo) InsertFromSkorozvon(ctx context.Context, in entity.LeadFromSkorozvon) error {
	skorozvonLeadID := strings.TrimSpace(in.SkorozvonLeadID)
	if skorozvonLeadID == "" {
		return fmt.Errorf("%w: skorozvon lead id is required", domain.ErrInvalidInput)
	}
	uuid, err := id.NewUUID()
	if err != nil {
		return fmt.Errorf("lead uuid: %w", err)
	}

	rawPhone := strings.TrimSpace(in.Phone)
	phoneVal := rawPhone
	phoneValid := false
	if norm, ok := phone.Normalize(rawPhone); ok {
		phoneVal = norm
		phoneValid = true
	}

	created := in.CreatedAt
	if created.IsZero() {
		created = clock.Now()
	} else {
		created = created.In(clock.Location)
	}

	var leadID int64
	err = r.pool.QueryRow(ctx, `
		INSERT INTO lead_row (
			uuid, phone, phone_valid, city, tag,
			skorozvon_lead_id, status_id, via_us, source_id, region_id, region_source,
			legacy_fields, created_at, updated_at
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7, false, NULL, $10::int, CASE WHEN $10::int IS NOT NULL THEN $11::smallint ELSE NULL END, NULLIF($8, ''), $9, now())
		ON CONFLICT (skorozvon_lead_id) WHERE skorozvon_lead_id IS NOT NULL DO NOTHING
		RETURNING id
	`, uuid, phoneVal, phoneValid, strings.TrimSpace(in.City), strings.TrimSpace(in.Tag),
		skorozvonLeadID, entity.StatusFromSK, strings.TrimSpace(in.LegacyRaw), created, in.RegionID, in.RegionSource).Scan(&leadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("insert lead from skorozvon: %w", err)
	}
	return insertLeadEvent(ctx, r.pool, leadID, entity.EventFromSK, entity.EventSourceSK, skorozvonLeadID)
}

func (r *LeadRepo) InsertDispatch(ctx context.Context, in entity.Dispatch) error {
	if strings.TrimSpace(in.SkorozvonProjectID) != "" {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO dispatch (lead_row_id, channel_id, skorozvon_project_id, external_id, sent_at, sent_ok, attempts, http_status, raw_response)
			VALUES ($1, $2, $3, NULLIF($4, ''), now(), $5, $6, $7, NULLIF($8, ''))
			ON CONFLICT (lead_row_id, channel_id, skorozvon_project_id) WHERE skorozvon_project_id IS NOT NULL
			DO UPDATE SET
				sent_at = EXCLUDED.sent_at,
				sent_ok = EXCLUDED.sent_ok,
				external_id = EXCLUDED.external_id,
				attempts = EXCLUDED.attempts,
				http_status = EXCLUDED.http_status,
				raw_response = EXCLUDED.raw_response
		`, in.LeadRowID, in.ChannelID, in.SkorozvonProjectID, in.ExternalID, in.SentOK, in.Attempts, in.HTTPStatus, in.RawResponse)
		if err != nil {
			return fmt.Errorf("insert skorozvon dispatch: %w", err)
		}
		return nil
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO dispatch (lead_row_id, channel_id, upk_id, sent_at, sent_ok, attempts, http_status, raw_response)
		VALUES ($1, $2, $3, now(), $4, $5, $6, NULLIF($7, ''))
		ON CONFLICT (lead_row_id, channel_id, upk_id) WHERE upk_id IS NOT NULL
		DO UPDATE SET
			sent_at = EXCLUDED.sent_at,
			sent_ok = EXCLUDED.sent_ok,
			attempts = EXCLUDED.attempts,
			http_status = EXCLUDED.http_status,
			raw_response = EXCLUDED.raw_response
	`, in.LeadRowID, in.ChannelID, in.UPKID, in.SentOK, in.Attempts, in.HTTPStatus, in.RawResponse)
	if err != nil {
		return fmt.Errorf("insert dispatch: %w", err)
	}
	return nil
}

func (r *LeadRepo) SaveZaryaResult(ctx context.Context, leadID int64, duplicate bool) error {
	name := "Чистый"
	if duplicate {
		name = "Дубль"
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO dedup_result (lead_row_id, provider_id, value_id, checked_at)
		SELECT $1, p.id, v.id, now()
		FROM dedup_provider p
		JOIN dedup_value v ON v.provider_id = p.id AND v.name = $2
		WHERE lower(p.name) IN ('заря', 'zarya')
	`, leadID, name)
	if err != nil {
		return fmt.Errorf("save zarya result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("save zarya result: provider or value %q not found", name)
	}
	return nil
}

func (r *LeadRepo) List(ctx context.Context, f port.LeadListFilter) ([]entity.LeadListItem, int, error) {
	where, args := leadListWhere(f)

	var total int
	countSQL := `SELECT count(*) FROM lead_row lr` + leadListSourceJoin(f) + `
` + where
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count leads: %w", err)
	}

	limit := f.Limit
	offset := f.Offset
	args = append(args, limit, offset)
	lim := len(args) - 1
	off := len(args)

	listSQL := fmt.Sprintf(`
WITH page AS (
	SELECT lr.id
	FROM lead_row lr
	%s
	%s
	ORDER BY lr.created_at DESC
	LIMIT $%d OFFSET $%d
)
SELECT
	lr.id,
	lr.uuid,
	lr.created_at,
	COALESCE(ps.name, ''),
	COALESCE(cs.name, ''),
	COALESCE(NULLIF(lr.cold_label, ''), cs.cold_label, ps.cold_label, ''),
	COALESCE(NULLIF(lr.base_label, ''), cs.base_label, ps.base_label, ''),
	COALESCE(lr.source_description, ''),
	COALESCE(r.name, ''),
	COALESCE(lr.city, ''),
	COALESCE(o.name, ''),
	st.id,
	st.name,
	lr.call_result,
	COALESCE(lr.call_result_name, ''),
	COALESCE(u.name, ''),
	lr.phone,
	COALESCE(z.name, ''),
	COALESCE(lr.tag, ''),
	COALESCE(lr.web_id, ''),
	COALESCE(lr.project_num, ''),
	COALESCE(lr.call_id, ''),
	COALESCE(lr.call_url, ''),
	lr.via_us
FROM page
JOIN lead_row lr ON lr.id = page.id
LEFT JOIN source ps ON ps.id = lr.source_id
LEFT JOIN source cs ON cs.id = lr.child_source_id
JOIN status st ON st.id = lr.status_id
LEFT JOIN region r ON r.id = lr.region_id
LEFT JOIN operator o ON o.id = lr.operator_id
LEFT JOIN LATERAL (
	SELECT d.upk_id
	FROM dispatch d
	WHERE d.lead_row_id = lr.id AND d.channel_id = 2
	ORDER BY d.sent_at DESC
	LIMIT 1
) disp ON true
LEFT JOIN upk u ON u.id = disp.upk_id
LEFT JOIN LATERAL (
	SELECT dv.name
	FROM dedup_result dr
	JOIN dedup_value dv ON dv.id = dr.value_id
	WHERE dr.lead_row_id = lr.id
	  AND dr.provider_id IN (SELECT p.id FROM dedup_provider p WHERE lower(p.name) IN ('заря', 'zarya'))
	ORDER BY dr.checked_at DESC
	LIMIT 1
) z ON true
ORDER BY lr.created_at DESC
`, leadListSourceJoin(f), where, lim, off)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadListItem, 0, limit)
	for rows.Next() {
		var item entity.LeadListItem
		if err := rows.Scan(
			&item.ID,
			&item.UUID,
			&item.DateCreate,
			&item.Source,
			&item.ChildSource,
			&item.ColdLabel,
			&item.BaseLabel,
			&item.SourceDescription,
			&item.Region,
			&item.City,
			&item.Operator,
			&item.StatusID,
			&item.Stage,
			&item.CallResult,
			&item.CallResultName,
			&item.UPK,
			&item.Phone,
			&item.Zarya,
			&item.Tag,
			&item.WebID,
			&item.ProjectNum,
			&item.CallID,
			&item.CallURL,
			&item.ViaUs,
		); err != nil {
			return nil, 0, fmt.Errorf("scan lead: %w", err)
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (r *LeadRepo) CountBySource(ctx context.Context, from, to time.Time) ([]entity.LeadSourceCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT source_id, COUNT(*)::int
		FROM lead_row
		WHERE created_at >= $1 AND created_at <= $2
		  AND source_id IS NOT NULL
		GROUP BY source_id
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("count leads by source: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadSourceCount, 0)
	for rows.Next() {
		var item entity.LeadSourceCount
		if err := rows.Scan(&item.SourceID, &item.Count); err != nil {
			return nil, fmt.Errorf("scan lead source count: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LeadRepo) CountEnrichmentBySource(ctx context.Context, from, to time.Time) ([]entity.LeadEnrichmentStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT source_id,
		       COUNT(*)::int,
		       COUNT(*) FILTER (WHERE region_source = 1)::int,
		       COUNT(*) FILTER (WHERE region_source = 2)::int,
		       COUNT(*) FILTER (WHERE COALESCE(region_source, 0) NOT IN (1, 2))::int,
		       COUNT(*) FILTER (WHERE operator_source = 1)::int,
		       COUNT(*) FILTER (WHERE operator_source = 2)::int,
		       COUNT(*) FILTER (WHERE COALESCE(operator_source, 0) NOT IN (1, 2))::int
		FROM lead_row
		WHERE created_at >= $1 AND created_at <= $2
		  AND source_id IS NOT NULL
		GROUP BY source_id
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("count enrichment by source: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadEnrichmentStat, 0)
	for rows.Next() {
		var item entity.LeadEnrichmentStat
		if err := rows.Scan(
			&item.SourceID,
			&item.Total,
			&item.Region.Supplier,
			&item.Region.Provider,
			&item.Region.Unknown,
			&item.Operator.Supplier,
			&item.Operator.Provider,
			&item.Operator.Unknown,
		); err != nil {
			return nil, fmt.Errorf("scan enrichment stat: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LeadRepo) CountZaryaBySource(ctx context.Context, from, to time.Time) ([]entity.LeadZaryaStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT lr.source_id,
		       COUNT(*)::int,
		       COUNT(*) FILTER (WHERE dv.name = 'Чистый')::int,
		       COUNT(*) FILTER (WHERE dv.name IS DISTINCT FROM 'Чистый')::int
		FROM lead_row lr
		JOIN LATERAL (
			SELECT dr.value_id
			FROM dedup_result dr
			WHERE dr.lead_row_id = lr.id
			  AND dr.provider_id = (
				SELECT p.id FROM dedup_provider p
				WHERE lower(p.name) IN ('заря', 'zarya')
				LIMIT 1
			  )
			ORDER BY dr.checked_at DESC
			LIMIT 1
		) z ON true
		JOIN dedup_value dv ON dv.id = z.value_id
		WHERE lr.created_at >= $1 AND lr.created_at <= $2
		  AND lr.source_id IS NOT NULL
		GROUP BY lr.source_id
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("count zarya by source: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadZaryaStat, 0)
	for rows.Next() {
		var item entity.LeadZaryaStat
		if err := rows.Scan(&item.SourceID, &item.Total, &item.Unique, &item.Dup); err != nil {
			return nil, fmt.Errorf("scan zarya stat: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LeadRepo) CountSkorozvonBitrixBySource(ctx context.Context, from, to time.Time) ([]entity.LeadSkorozvonBitrixStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(source_id, 0),
		       COUNT(*) FILTER (
		           WHERE via_us
		             AND COALESCE(btrim(skorozvon_lead_id), '') <> ''
		       )::int,
		       COUNT(*) FILTER (
		           WHERE via_us
		             AND COALESCE(btrim(skorozvon_lead_id), '') <> ''
		             AND COALESCE(btrim(bitrix_lead_id), '') <> ''
		       )::int,
		       COUNT(*) FILTER (WHERE NOT via_us)::int,
		       COUNT(*) FILTER (
		           WHERE NOT via_us
		             AND COALESCE(btrim(bitrix_lead_id), '') <> ''
		       )::int
		FROM lead_row
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY COALESCE(source_id, 0)
		HAVING COUNT(*) FILTER (
		           WHERE via_us AND COALESCE(btrim(skorozvon_lead_id), '') <> ''
		       ) > 0
		    OR COUNT(*) FILTER (WHERE NOT via_us) > 0
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("count skorozvon bitrix by source: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadSkorozvonBitrixStat, 0)
	for rows.Next() {
		var item entity.LeadSkorozvonBitrixStat
		if err := rows.Scan(&item.SourceID, &item.Updated, &item.UpdatedSent, &item.Created, &item.CreatedSent); err != nil {
			return nil, fmt.Errorf("scan skorozvon bitrix stat: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

const leadDetailSelect = `
SELECT
	lr.id,
	lr.uuid,
	lr.phone,
	lr.phone_valid,
	lr.via_us,
	lr.source_id,
	COALESCE(ps.name, ''),
	lr.child_source_id,
	COALESCE(cs.name, ''),
	COALESCE(NULLIF(lr.cold_label, ''), s.cold_label, ''),
	COALESCE(NULLIF(lr.base_label, ''), s.base_label, ''),
	lr.region_id,
	COALESCE(reg.name, ''),
	lr.region_source,
	COALESCE(lr.city, ''),
	lr.operator_id,
	COALESCE(op.name, ''),
	lr.operator_source,
	COALESCE(lr.web_id, ''),
	COALESCE(lr.tag, ''),
	lr.call_result,
	COALESCE(lr.call_result_name, ''),
	COALESCE(lr.call_id, ''),
	COALESCE(lr.call_url, ''),
	lr.call_last_at,
	COALESCE(lr.project_num, ''),
	lr.call_form,
	lr.status_id,
	st.name,
	COALESCE(lr.s3_key, ''),
	lr.legacy_fields,
	COALESCE(lr.bitrix_lead_id, ''),
	COALESCE(lr.source_description, ''),
	lr.archived_at,
	lr.created_at,
	lr.updated_at,
	COALESCE(z.name, ''),
	COALESCE(u.name, '')
FROM lead_row lr
LEFT JOIN source s ON s.id = COALESCE(lr.child_source_id, lr.source_id)
LEFT JOIN source ps ON ps.id = lr.source_id
LEFT JOIN source cs ON cs.id = lr.child_source_id
JOIN status st ON st.id = lr.status_id
LEFT JOIN region reg ON reg.id = lr.region_id
LEFT JOIN operator op ON op.id = lr.operator_id
LEFT JOIN LATERAL (
	SELECT d.upk_id
	FROM dispatch d
	WHERE d.lead_row_id = lr.id AND d.channel_id = 2
	ORDER BY d.sent_at DESC
	LIMIT 1
) disp ON true
LEFT JOIN upk u ON u.id = disp.upk_id
LEFT JOIN LATERAL (
	SELECT dv.name
	FROM dedup_result dr
	JOIN dedup_value dv ON dv.id = dr.value_id
	WHERE dr.lead_row_id = lr.id
	  AND dr.provider_id IN (SELECT p.id FROM dedup_provider p WHERE lower(p.name) IN ('заря', 'zarya'))
	ORDER BY dr.checked_at DESC
	LIMIT 1
) z ON true
`

func (r *LeadRepo) Get(ctx context.Context, id int64) (entity.LeadDetail, error) {
	d, err := r.getLead(ctx, "lr.id = $1", id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.LeadDetail{}, fmt.Errorf("%w: lead %d", domain.ErrNotFound, id)
		}
		return entity.LeadDetail{}, err
	}
	return d, nil
}

func (r *LeadRepo) GetByUUID(ctx context.Context, uuid string) (entity.LeadDetail, error) {
	d, err := r.getLead(ctx, "lr.uuid = $1", uuid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.LeadDetail{}, fmt.Errorf("%w: lead %s", domain.ErrNotFound, uuid)
		}
		return entity.LeadDetail{}, err
	}
	return d, nil
}

func (r *LeadRepo) getLead(ctx context.Context, where string, arg any) (entity.LeadDetail, error) {
	q := leadDetailSelect + "WHERE " + where
	var d entity.LeadDetail
	var callForm, legacy *string
	err := r.pool.QueryRow(ctx, q, arg).Scan(
		&d.ID,
		&d.UUID,
		&d.Phone,
		&d.PhoneValid,
		&d.ViaUs,
		&d.SourceID,
		&d.Source,
		&d.ChildSourceID,
		&d.ChildSource,
		&d.ColdLabel,
		&d.BaseLabel,
		&d.RegionID,
		&d.Region,
		&d.RegionSource,
		&d.City,
		&d.OperatorID,
		&d.Operator,
		&d.OperatorSource,
		&d.WebID,
		&d.Tag,
		&d.CallResult,
		&d.CallResultName,
		&d.CallID,
		&d.CallURL,
		&d.CallLastAt,
		&d.ProjectNum,
		&callForm,
		&d.StatusID,
		&d.Stage,
		&d.S3Key,
		&legacy,
		&d.BitrixLeadID,
		&d.SourceDescription,
		&d.ArchivedAt,
		&d.CreatedAt,
		&d.UpdatedAt,
		&d.Zarya,
		&d.UPK,
	)
	if err != nil {
		return entity.LeadDetail{}, err
	}
	d.CallFormRaw = deref(callForm)
	d.LegacyRaw = deref(legacy)

	events, err := r.listEvents(ctx, d.ID)
	if err != nil {
		return entity.LeadDetail{}, err
	}
	d.Events = events

	dispatches, err := r.listDispatches(ctx, d.ID)
	if err != nil {
		return entity.LeadDetail{}, err
	}
	d.Dispatches = dispatches

	history, err := r.listPhoneHistory(ctx, d.Phone, d.ID)
	if err != nil {
		return entity.LeadDetail{}, err
	}
	d.PhoneHistory = history

	anketa, err := r.getAnketa(ctx, d.ID)
	if err != nil {
		return entity.LeadDetail{}, err
	}
	d.Anketa = anketa

	fields, err := r.listAnketaFields(ctx, d.ID)
	if err != nil {
		return entity.LeadDetail{}, err
	}
	d.AnketaFields = fields
	return d, nil
}

func (r *LeadRepo) listAnketaFields(ctx context.Context, leadID int64) ([]entity.SkorozvonAnketaFieldItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			f.field_id,
			COALESCE(mf.name, ''),
			COALESCE(mf.bitrix_code, ''),
			f.enum_id,
			COALESCE(mo.name, '')
		FROM skorozvon_anketa_fields f
		LEFT JOIN skorozvon_maps mf ON mf.field_id = f.field_id AND mf.option_id = 0
		LEFT JOIN skorozvon_maps mo ON mo.field_id = f.field_id AND mo.option_id = f.enum_id
		WHERE f.lead_row_id = $1
		ORDER BY f.id
	`, leadID)
	if err != nil {
		return nil, fmt.Errorf("list anketa fields: %w", err)
	}
	defer rows.Close()

	out := make([]entity.SkorozvonAnketaFieldItem, 0)
	for rows.Next() {
		var item entity.SkorozvonAnketaFieldItem
		if err := rows.Scan(&item.FieldID, &item.FieldName, &item.BitrixCode, &item.EnumID, &item.EnumName); err != nil {
			return nil, fmt.Errorf("scan anketa field: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LeadRepo) getAnketa(ctx context.Context, leadID int64) (*entity.SkorozvonAnketa, error) {
	var (
		out     entity.SkorozvonAnketa
		answers string
	)
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, form_id, scenario_id, COALESCE(form_name, ''), COALESCE(call_id, ''), answers, form_updated_at
		FROM skorozvon_anketa
		WHERE lead_row_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, leadID).Scan(
		&out.ID,
		&out.UserID,
		&out.FormID,
		&out.ScenarioID,
		&out.FormName,
		&out.CallID,
		&answers,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get anketa: %w", err)
	}
	if strings.TrimSpace(answers) != "" {
		if err := json.Unmarshal([]byte(answers), &out.Answers); err != nil {
			return nil, fmt.Errorf("decode anketa answers: %w", err)
		}
	}
	if out.Answers == nil {
		out.Answers = []entity.SkorozvonAnketaAnswer{}
	}
	return &out, nil
}

func (r *LeadRepo) listEvents(ctx context.Context, leadID int64) ([]entity.LeadEvent, error) {
	rows, err := r.pool.Query(ctx, `
SELECT e.event_type_id, et.name, e.occurred_at, e.source, COALESCE(e.details, '')
FROM event e
JOIN event_type et ON et.id = e.event_type_id
WHERE e.lead_row_id = $1
ORDER BY e.occurred_at, e.id
`, leadID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadEvent, 0)
	for rows.Next() {
		var item entity.LeadEvent
		if err := rows.Scan(&item.TypeID, &item.TypeName, &item.OccurredAt, &item.Source, &item.Details); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LeadRepo) listDispatches(ctx context.Context, leadID int64) ([]entity.LeadDispatch, error) {
	rows, err := r.pool.Query(ctx, `
SELECT
	d.channel_id,
	c.name,
	d.upk_id,
	COALESCE(u.name, ''),
	COALESCE(d.skorozvon_project_id, ''),
	d.sent_at,
	d.sent_ok,
	COALESCE(d.external_id, ''),
	d.attempts,
	d.http_status,
	COALESCE(d.raw_response, '')
FROM dispatch d
JOIN channel c ON c.id = d.channel_id
LEFT JOIN upk u ON u.id = d.upk_id
WHERE d.lead_row_id = $1
ORDER BY d.sent_at, d.id
`, leadID)
	if err != nil {
		return nil, fmt.Errorf("list dispatch: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadDispatch, 0)
	for rows.Next() {
		var item entity.LeadDispatch
		if err := rows.Scan(
			&item.ChannelID,
			&item.Channel,
			&item.UPKID,
			&item.UPK,
			&item.ProjectID,
			&item.SentAt,
			&item.SentOK,
			&item.ExternalID,
			&item.Attempts,
			&item.HTTPStatus,
			&item.RawResponse,
		); err != nil {
			return nil, fmt.Errorf("scan dispatch: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LeadRepo) listPhoneHistory(ctx context.Context, phone string, exceptID int64) ([]entity.LeadPhoneHistory, error) {
	if phone == "" {
		return []entity.LeadPhoneHistory{}, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT lr.id, lr.uuid, lr.created_at, COALESCE(s.name, ''), lr.status_id, st.name
FROM lead_row lr
LEFT JOIN source s ON s.id = COALESCE(lr.child_source_id, lr.source_id)
JOIN status st ON st.id = lr.status_id
WHERE lr.phone = $1 AND lr.id <> $2
ORDER BY lr.created_at DESC
LIMIT 20
`, phone, exceptID)
	if err != nil {
		return nil, fmt.Errorf("list phone history: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadPhoneHistory, 0)
	for rows.Next() {
		var item entity.LeadPhoneHistory
		if err := rows.Scan(&item.ID, &item.UUID, &item.DateCreate, &item.Source, &item.StatusID, &item.Stage); err != nil {
			return nil, fmt.Errorf("scan phone history: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func leadListNeedsSourceJoin(f port.LeadListFilter) bool {
	return len(f.ColdLabels) > 0
}

func leadListSourceJoin(f port.LeadListFilter) string {
	if !leadListNeedsSourceJoin(f) {
		return ""
	}
	return `
JOIN source s ON s.id = COALESCE(lr.child_source_id, lr.source_id)`
}

func leadListWhere(f port.LeadListFilter) (string, []any) {
	var b strings.Builder
	args := make([]any, 0, 8)
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if len(f.IDs) > 0 {
		b.WriteString("WHERE lr.id = ANY(" + next(f.IDs) + "::bigint[])")
	} else {
		b.WriteString("WHERE lr.created_at >= " + next(f.DateFrom) + " AND lr.created_at <= " + next(f.DateTo))
	}

	if len(f.Sources) > 0 {
		p := next(intSlice(f.Sources))
		b.WriteString(" AND lr.source_id = ANY(" + p + "::int[])")
	}
	if len(f.ChildSources) > 0 {
		p := next(intSlice(f.ChildSources))
		b.WriteString(" AND lr.child_source_id = ANY(" + p + "::int[])")
	}
	if len(f.ColdLabels) > 0 {
		p := next(f.ColdLabels)
		b.WriteString(" AND COALESCE(NULLIF(lr.cold_label, ''), s.cold_label) = ANY(" + p + "::text[])")
	}
	if len(f.Regions) > 0 {
		p := next(intSlice(f.Regions))
		b.WriteString(" AND lr.region_id = ANY(" + p + "::int[])")
	}
	if len(f.Operators) > 0 {
		p := next(intSlice(f.Operators))
		b.WriteString(" AND lr.operator_id = ANY(" + p + "::int[])")
	}
	if len(f.Statuses) > 0 {
		p := next(intSlice(f.Statuses))
		b.WriteString(" AND lr.status_id = ANY(" + p + "::int[])")
	}
	if f.City != "" {
		b.WriteString(" AND lr.city ILIKE " + next("%"+f.City+"%"))
	}
	if f.Tag != "" {
		b.WriteString(" AND lr.tag ILIKE " + next("%"+f.Tag+"%"))
	}
	if f.CallResult != nil {
		b.WriteString(" AND lr.call_result = " + next(*f.CallResult))
	}
	if f.Phone != "" {
		b.WriteString(" AND lr.phone = " + next(f.Phone))
	}
	if f.ViaUs != nil {
		b.WriteString(" AND lr.via_us = " + next(*f.ViaUs))
	}
	if len(f.Zarya) > 0 {
		p := next(f.Zarya)
		b.WriteString(` AND (
	SELECT dv.name
	FROM dedup_result dr
	JOIN dedup_value dv ON dv.id = dr.value_id
	WHERE dr.lead_row_id = lr.id
	  AND dr.provider_id IN (SELECT p.id FROM dedup_provider p WHERE lower(p.name) IN ('заря', 'zarya'))
	ORDER BY dr.checked_at DESC
	LIMIT 1
) = ANY(` + p + `::text[])`)
	}
	return b.String(), args
}

func intSlice(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}

func nilIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

type dbExec interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertLeadEvent(ctx context.Context, exec dbExec, leadID int64, typeID, source int16, details string) error {
	_, err := exec.Exec(ctx, `
		INSERT INTO event (lead_row_id, event_type_id, occurred_at, source, details)
		VALUES ($1, $2, now(), $3, NULLIF($4, ''))
	`, leadID, typeID, source, details)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}
