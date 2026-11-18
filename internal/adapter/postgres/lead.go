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
	phoneValue := strings.TrimSpace(row.Phone)
	phoneValid := false
	if normalized, ok := phone.Normalize(phoneValue); ok {
		phoneValue = normalized
		phoneValid = true
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin accept txn: %w", err)
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO lead_row (uuid, phone, phone_valid, source_id, web_id, status_id, s3_key, source_description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, row.UUID, phoneValue, phoneValid, row.SourceID, nilIfEmpty(row.WebID), row.StatusID, row.S3Key, nilIfEmpty(row.SourceDescription), row.CreatedAt).Scan(&id)
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

var importLeadColumns = map[string]string{
	"uuid": "text", "phone": "text", "phone_valid": "bool",
	"source_id": "int", "child_source_id": "int", "region_id": "int",
	"region_source": "int16", "city": "text", "operator_id": "int",
	"operator_source": "int16", "operator_raw": "text", "web_id": "text",
	"tag": "text", "call_result": "int64", "call_result_name": "text",
	"call_id": "text", "call_url": "text", "call_last_at": "time",
	"project_num": "text", "call_form": "text", "status_id": "int16",
	"s3_key": "text", "archived_at": "time", "created_at": "time",
	"updated_at": "time", "legacy_fields": "text", "bitrix_lead_id": "text",
	"cold_label": "text", "base_label": "text", "source_description": "text",
	"skorozvon_lead_id": "text", "skorozvon_anketa_id": "int64", "via_us": "bool",
}

func (r *LeadRepo) InsertImport(ctx context.Context, in entity.LeadImportRow) error {
	if strings.TrimSpace(in.UUID) == "" || strings.TrimSpace(in.Phone) == "" {
		return fmt.Errorf("insert import: uuid and phone required")
	}
	if in.SourceID <= 0 || strings.TrimSpace(in.Payload) == "" {
		return fmt.Errorf("insert import: source_id and payload required")
	}

	cols := make([]string, 0, 8)
	args := make([]any, 0, 8)
	seen := map[string]bool{}
	add := func(col string, val any) {
		if seen[col] {
			return
		}
		seen[col] = true
		cols = append(cols, col)
		args = append(args, val)
	}
	add("uuid", in.UUID)
	add("phone", in.Phone)
	add("phone_valid", in.PhoneValid)
	for col, val := range in.Values {
		kind, ok := importLeadColumns[col]
		if !ok || col == "uuid" || col == "phone" || col == "phone_valid" {
			continue
		}
		if val == nil {
			continue
		}
		if col == "child_source_id" {
			continue
		}
		if kind == "text" {
			if s, ok := val.(string); ok && strings.TrimSpace(s) == "" {
				continue
			}
		}
		add(col, val)
	}
	add("source_id", in.SourceID)
	if !seen["status_id"] {
		add("status_id", entity.StatusAccepted)
	}
	if !seen["created_at"] {
		add("created_at", clock.Now())
	}

	placeholders := make([]string, len(args))
	for i := range args {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin import txn: %w", err)
	}
	defer tx.Rollback(ctx)

	q := fmt.Sprintf(
		`INSERT INTO lead_row (%s) VALUES (%s) RETURNING id`,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)
	var leadID int64
	if err := tx.QueryRow(ctx, q, args...).Scan(&leadID); err != nil {
		return fmt.Errorf("insert import lead_row: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (lead_row_id, source_id, payload, created_at)
		VALUES ($1, $2, $3, $4)
	`, leadID, in.SourceID, in.Payload, clock.Now()); err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}
	if err := insertLeadEvent(ctx, tx, leadID, entity.EventAccepted, entity.EventSourceAPI, ""); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit import txn: %w", err)
	}
	return nil
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

// resolveSkorozvonLeadSQL — lead_row.id по любому id СК ($1).
// Сначала наша заявка (via_us), иначе оригинал с тем же телефоном, иначе клон.
const resolveSkorozvonLeadSQL = `
	SELECT id FROM (
		SELECT COALESCE(
			(SELECT lr.id
			 FROM lead_skorozvon ls
			 JOIN lead_row lr ON lr.id = ls.lead_row_id
			 WHERE ls.skorozvon_lead_id = $1 AND lr.via_us
			 LIMIT 1),
			(SELECT id FROM lead_row WHERE skorozvon_lead_id = $1 AND via_us LIMIT 1),
			(SELECT orig.id
			 FROM lead_row clone
			 JOIN lead_row orig
			   ON orig.via_us
			  AND orig.phone = clone.phone
			  AND COALESCE(btrim(clone.phone), '') <> ''
			 WHERE NOT clone.via_us
			   AND clone.id IN (
					SELECT lead_row_id FROM lead_skorozvon WHERE skorozvon_lead_id = $1
					UNION ALL
					SELECT id FROM lead_row WHERE skorozvon_lead_id = $1
			   )
			   AND (
					EXISTS (SELECT 1 FROM lead_skorozvon ls WHERE ls.lead_row_id = orig.id)
					OR COALESCE(btrim(orig.skorozvon_lead_id), '') <> ''
			   )
			 ORDER BY orig.id DESC
			 LIMIT 1),
			(SELECT lead_row_id FROM lead_skorozvon WHERE skorozvon_lead_id = $1 LIMIT 1),
			(SELECT id FROM lead_row WHERE skorozvon_lead_id = $1 LIMIT 1)
		) AS id
	) s
	WHERE id IS NOT NULL
`

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
		),
		mapped AS (
			INSERT INTO lead_skorozvon (skorozvon_lead_id, lead_row_id, project_id)
			SELECT NULLIF($2, ''), id, NULLIF($3, '')
			FROM upd
			WHERE NULLIF($2, '') IS NOT NULL
			ON CONFLICT (skorozvon_lead_id) DO NOTHING
		)
		INSERT INTO event (lead_row_id, event_type_id, occurred_at, source, details)
		SELECT id, $5, now(), $6, NULLIF($2, '') FROM upd
	`, leadID, skorozvonLeadID, projectID, entity.StatusInSK, entity.EventInSK, entity.EventSourceWorker)
	if err != nil {
		return fmt.Errorf("bind skorozvon: %w", err)
	}
	return nil
}

func (r *LeadRepo) BindCrmcalls(ctx context.Context, leadID int64, contactID, projectID string) error {
	contactID = strings.TrimSpace(contactID)
	projectID = strings.TrimSpace(projectID)
	_, err := r.pool.Exec(ctx, `
		WITH upd AS (
			UPDATE lead_row
			SET status_id = $4, updated_at = now()
			WHERE id = $1
			RETURNING id
		),
		mapped AS (
			INSERT INTO lead_crmcalls (contact_id, lead_row_id, project_id)
			SELECT NULLIF($2, ''), id, NULLIF($3, '')
			FROM upd
			WHERE NULLIF($2, '') IS NOT NULL
			ON CONFLICT (contact_id) DO NOTHING
		)
		INSERT INTO event (lead_row_id, event_type_id, occurred_at, source, details)
		SELECT id, $5, now(), $6, NULLIF($2, '') FROM upd
	`, leadID, contactID, projectID, entity.StatusInSK, entity.EventInSK, entity.EventSourceWorker)
	if err != nil {
		return fmt.Errorf("bind crmcalls: %w", err)
	}
	return nil
}

func (r *LeadRepo) HasCrmcallsSent(ctx context.Context, leadID int64, projectID string) (bool, error) {
	projectID = strings.TrimSpace(projectID)
	if leadID <= 0 || projectID == "" {
		return false, nil
	}
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM dispatch
			WHERE lead_row_id = $1
			  AND channel_id = $3
			  AND sent_ok
			  AND skorozvon_project_id = $2
		) OR EXISTS (
			SELECT 1 FROM lead_crmcalls
			WHERE lead_row_id = $1
			  AND project_id = $2
		)
	`, leadID, projectID, entity.ChannelCrmcalls).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("has crmcalls sent: %w", err)
	}
	return ok, nil
}

func (r *LeadRepo) ApplyCrmcallsPostback(ctx context.Context, in entity.CrmcallsPostback) error {
	id := strings.TrimSpace(in.ID)
	phoneVal := strings.TrimSpace(in.Phone)
	if n, ok := phone.Normalize(phoneVal); ok {
		phoneVal = n
	}
	result := strings.TrimSpace(in.Result)
	if id == "" && phoneVal == "" {
		return fmt.Errorf("%w: id or phone is required", domain.ErrInvalidInput)
	}

	var leadID int64
	var leadUUID string
	err := r.pool.QueryRow(ctx, `
		SELECT lr.id, lr.uuid
		FROM lead_row lr
		WHERE (
			($1 <> '' AND lr.uuid = $1)
			OR ($1 <> '' AND lr.id::text = $1)
			OR ($1 <> '' AND EXISTS (
				SELECT 1 FROM lead_crmcalls c WHERE c.lead_row_id = lr.id AND c.contact_id = $1
			))
			OR ($2 <> '' AND lr.phone = $2 AND lr.via_us)
		)
		ORDER BY
			CASE
				WHEN $1 <> '' AND lr.uuid = $1 THEN 0
				WHEN $1 <> '' AND lr.id::text = $1 THEN 1
				WHEN $1 <> '' AND EXISTS (
					SELECT 1 FROM lead_crmcalls c WHERE c.lead_row_id = lr.id AND c.contact_id = $1
				) THEN 2
				ELSE 3
			END,
			lr.id DESC
		LIMIT 1
	`, id, phoneVal).Scan(&leadID, &leadUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("resolve crmcalls postback: %w", err)
	}

	name := entity.UsableSkorozvonName(in.Name)
	callForm := ""
	if name != "" {
		var current string
		_ = r.pool.QueryRow(ctx, `SELECT COALESCE(call_form, '') FROM lead_row WHERE id = $1`, leadID).Scan(&current)
		callForm = mergeCallFormImya(current, name)
	}

	callID := strings.TrimSpace(in.CallID)
	callURL := strings.TrimSpace(in.CallURL)
	if _, err := r.pool.Exec(ctx, `
		UPDATE lead_row
		SET call_result_name = COALESCE(NULLIF($2, ''), call_result_name),
		    call_form = COALESCE(NULLIF($3, ''), call_form),
		    call_id = COALESCE(NULLIF($4, ''), call_id),
		    call_url = COALESCE(NULLIF($5, ''), call_url),
		    call_last_at = COALESCE($6, call_last_at),
		    updated_at = now()
		WHERE id = $1
	`, leadID, result, callForm, callID, callURL, in.CallLastAt); err != nil {
		return fmt.Errorf("apply crmcalls result: %w", err)
	}

	if id != "" && id != leadUUID {
		if _, err := r.pool.Exec(ctx, `
			INSERT INTO lead_crmcalls (contact_id, lead_row_id)
			VALUES ($1, $2)
			ON CONFLICT (contact_id) DO NOTHING
		`, id, leadID); err != nil {
			return fmt.Errorf("map crmcalls contact: %w", err)
		}
	}

	if !crmcallsTransferred(result) {
		return nil
	}
	return r.enqueueBx24LeadID(ctx, leadID)
}

func crmcallsTransferred(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "передан лид", "заказ/лид":
		return true
	default:
		return false
	}
}

func mergeCallFormImya(raw, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return strings.TrimSpace(raw)
	}
	m := map[string]any{}
	if s := strings.TrimSpace(raw); s != "" {
		if err := json.Unmarshal([]byte(s), &m); err != nil || m == nil {
			m = map[string]any{}
		}
	}
	m["imya"] = name
	b, err := json.Marshal(m)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return string(b)
}

func (r *LeadRepo) enqueueBx24LeadID(ctx context.Context, leadID int64) error {
	outboxID, err := id.NewUUID()
	if err != nil {
		return fmt.Errorf("outbox_bx24 id: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		WITH lead AS (
			SELECT id, uuid
			FROM lead_row
			WHERE id = $1
			  AND via_us
			  AND COALESCE(bitrix_lead_id, '') = ''
		),
		upd AS (
			UPDATE lead_row lr
			SET status_id = $2, updated_at = now()
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
		)
		INSERT INTO event (lead_row_id, event_type_id, occurred_at, source)
		SELECT id, $4, now(), $5
		FROM upd
		JOIN queued ON queued.uuid_lead = upd.uuid
	`, leadID, entity.StatusFromSK, outboxID, entity.EventFromSK, entity.EventSourceSK)
	if err != nil {
		return fmt.Errorf("enqueue bx24 from crmcalls: %w", err)
	}
	return nil
}

func (r *LeadRepo) HasSkorozvonSent(ctx context.Context, leadID int64, projectID string) (bool, error) {
	projectID = strings.TrimSpace(projectID)
	if leadID <= 0 || projectID == "" {
		return false, nil
	}
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM dispatch
			WHERE lead_row_id = $1
			  AND channel_id = $3
			  AND sent_ok
			  AND skorozvon_project_id = $2
		) OR EXISTS (
			SELECT 1 FROM lead_skorozvon
			WHERE lead_row_id = $1
			  AND project_id = $2
		)
	`, leadID, projectID, entity.ChannelSkorozvon).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("has skorozvon sent: %w", err)
	}
	return ok, nil
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
		WHERE id = (`+resolveSkorozvonLeadSQL+`)
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
		    region_id = COALESCE($3, region_id),
		    region_source = CASE
		        WHEN $3 IS NOT NULL THEN $4
		        ELSE region_source
		    END,
		    updated_at = now()
		WHERE id = (`+resolveSkorozvonLeadSQL+`)
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
	err = tx.QueryRow(ctx, resolveSkorozvonLeadSQL, skorozvonLeadID).Scan(&leadRowID)
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
		if f.FieldID <= 0 {
			continue
		}
		text := strings.TrimSpace(f.ValueText)
		if f.EnumID <= 0 && text == "" {
			continue
		}
		enumID := f.EnumID
		if enumID < 0 {
			enumID = 0
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO skorozvon_anketa_fields (lead_row_id, skorozvon_lead_id, field_id, enum_id, value_text)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		`, leadRowID, skorozvonLeadID, f.FieldID, enumID, text); err != nil {
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
			SELECT lr.id, lr.uuid
			FROM lead_row lr
			WHERE lr.id = (`+resolveSkorozvonLeadSQL+`)
			  AND lr.via_us
			  AND COALESCE(lr.bitrix_lead_id, '') = ''
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
			SELECT lr.id, lr.uuid, lr.via_us
			FROM lead_row lr
			WHERE lr.id = (`+resolveSkorozvonLeadSQL+`)
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

	var existingID int64
	err = r.pool.QueryRow(ctx, resolveSkorozvonLeadSQL, skorozvonLeadID).Scan(&existingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("lookup skorozvon lead: %w", err)
	}
	if err == nil {
		return mapSkorozvonLead(ctx, r.pool, skorozvonLeadID, existingID, "")
	}
	if phoneValid {
		err = r.pool.QueryRow(ctx, `
			SELECT id
			FROM lead_row
			WHERE via_us
			  AND phone = $1
			  AND (
			    EXISTS (SELECT 1 FROM lead_skorozvon ls WHERE ls.lead_row_id = lead_row.id)
			    OR COALESCE(btrim(skorozvon_lead_id), '') <> ''
			  )
			ORDER BY id DESC
			LIMIT 1
		`, phoneVal).Scan(&existingID)
		if err == nil {
			return mapSkorozvonLead(ctx, r.pool, skorozvonLeadID, existingID, "")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("lookup skorozvon lead by phone: %w", err)
		}
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
	if err := mapSkorozvonLead(ctx, r.pool, skorozvonLeadID, leadID, ""); err != nil {
		return err
	}
	return insertLeadEvent(ctx, r.pool, leadID, entity.EventFromSK, entity.EventSourceSK, skorozvonLeadID)
}

func mapSkorozvonLead(ctx context.Context, exec interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}, skorozvonLeadID string, leadID int64, projectID string) error {
	_, err := exec.Exec(ctx, `
		INSERT INTO lead_skorozvon (skorozvon_lead_id, lead_row_id, project_id)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (skorozvon_lead_id) DO UPDATE SET
			lead_row_id = EXCLUDED.lead_row_id,
			project_id = COALESCE(NULLIF(EXCLUDED.project_id, ''), lead_skorozvon.project_id)
		WHERE EXISTS (
			SELECT 1 FROM lead_row n WHERE n.id = EXCLUDED.lead_row_id AND n.via_us
		) AND NOT EXISTS (
			SELECT 1 FROM lead_row o WHERE o.id = lead_skorozvon.lead_row_id AND o.via_us
		)
	`, skorozvonLeadID, leadID, projectID)
	if err != nil {
		return fmt.Errorf("map skorozvon lead: %w", err)
	}
	return nil
}

func (r *LeadRepo) InsertDispatch(ctx context.Context, in entity.Dispatch) error {
	if strings.TrimSpace(in.SkorozvonProjectID) != "" {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO dispatch (lead_row_id, channel_id, skorozvon_project_id, external_id, sent_at, sent_ok, attempts, http_status, raw_response)
			VALUES ($1, $2, $3, NULLIF($4, ''), now(), $5, $6, $7, NULLIF($8, ''))
			ON CONFLICT (lead_row_id, channel_id, skorozvon_project_id) WHERE skorozvon_project_id IS NOT NULL
			DO UPDATE SET
				sent_at = CASE WHEN dispatch.sent_ok THEN dispatch.sent_at ELSE EXCLUDED.sent_at END,
				sent_ok = dispatch.sent_ok OR EXCLUDED.sent_ok,
				external_id = CASE
					WHEN dispatch.sent_ok AND COALESCE(btrim(dispatch.external_id), '') <> '' THEN dispatch.external_id
					ELSE EXCLUDED.external_id
				END,
				attempts = EXCLUDED.attempts,
				http_status = CASE WHEN dispatch.sent_ok THEN dispatch.http_status ELSE EXCLUDED.http_status END,
				raw_response = CASE WHEN dispatch.sent_ok THEN dispatch.raw_response ELSE EXCLUDED.raw_response END
		`, in.LeadRowID, in.ChannelID, in.SkorozvonProjectID, in.ExternalID, in.SentOK, in.Attempts, in.HTTPStatus, in.RawResponse)
		if err != nil {
			return fmt.Errorf("insert skorozvon dispatch: %w", err)
		}
		return nil
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO dispatch (lead_row_id, channel_id, upk_id, external_id, sent_at, sent_ok, attempts, http_status, raw_response)
		VALUES ($1, $2, $3, NULLIF($4, ''), now(), $5, $6, $7, NULLIF($8, ''))
		ON CONFLICT (lead_row_id, channel_id, upk_id) WHERE upk_id IS NOT NULL
		DO UPDATE SET
			sent_at = EXCLUDED.sent_at,
			sent_ok = EXCLUDED.sent_ok,
			external_id = CASE
				WHEN COALESCE(btrim(dispatch.external_id), '') <> '' THEN dispatch.external_id
				ELSE EXCLUDED.external_id
			END,
			attempts = EXCLUDED.attempts,
			http_status = EXCLUDED.http_status,
			raw_response = EXCLUDED.raw_response
	`, in.LeadRowID, in.ChannelID, in.UPKID, in.ExternalID, in.SentOK, in.Attempts, in.HTTPStatus, in.RawResponse)
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

func (r *LeadRepo) SaveZaryaOldResult(ctx context.Context, leadID int64, duplicate bool) error {
	name := "Чистый"
	if duplicate {
		name = "дубль"
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO dedup_result (lead_row_id, provider_id, value_id, checked_at)
		SELECT $1, p.id, v.id, now()
		FROM dedup_provider p
		JOIN dedup_value v ON v.provider_id = p.id AND v.name = $2
		WHERE lower(p.name) IN ('заря старая', 'zarya_old')
	`, leadID, name)
	if err != nil {
		return fmt.Errorf("save zarya old result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("save zarya old result: provider or value %q not found", name)
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
	COALESCE(zo.name, ''),
	COALESCE(lr.tag, ''),
	COALESCE(lr.web_id, ''),
	COALESCE(lr.project_num, ''),
	COALESCE(lr.call_id, ''),
	COALESCE(lr.call_url, ''),
	lr.via_us,
	%s
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
LEFT JOIN LATERAL (
	SELECT dv.name
	FROM dedup_result dr
	JOIN dedup_value dv ON dv.id = dr.value_id
	WHERE dr.lead_row_id = lr.id
	  AND dr.provider_id IN (SELECT p.id FROM dedup_provider p WHERE lower(p.name) IN ('заря старая', 'zarya_old'))
	ORDER BY dr.checked_at DESC
	LIMIT 1
) zo ON true
%s
ORDER BY lr.created_at DESC
`, leadListSourceJoin(f), where, lim, off, leadListSkSelect(f), leadListSkJoin(f))

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
			&item.ZaryaOld,
			&item.Tag,
			&item.WebID,
			&item.ProjectNum,
			&item.CallID,
			&item.CallURL,
			&item.ViaUs,
			&item.SkSentOK,
			&item.SkHTTPStatus,
			&item.SkError,
		); err != nil {
			return nil, 0, fmt.Errorf("scan lead: %w", err)
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (r *LeadRepo) ListSkDispatch(ctx context.Context, ids []int64) ([]entity.LeadSkDispatch, error) {
	if len(ids) == 0 {
		return []entity.LeadSkDispatch{}, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT ON (d.lead_row_id)
	d.lead_row_id,
	d.sent_ok,
	d.http_status,
	LEFT(COALESCE(d.raw_response, ''), 500)
FROM dispatch d
WHERE d.channel_id = $1 AND d.lead_row_id = ANY($2)
ORDER BY d.lead_row_id, d.sent_at DESC
`, entity.ChannelSkorozvon, ids)
	if err != nil {
		return nil, fmt.Errorf("list skorozvon dispatch: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadSkDispatch, 0, len(ids))
	for rows.Next() {
		var item entity.LeadSkDispatch
		if err := rows.Scan(&item.LeadRowID, &item.SentOK, &item.HTTPStatus, &item.Error); err != nil {
			return nil, fmt.Errorf("scan skorozvon dispatch: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LeadRepo) ListSkResend(ctx context.Context, ids []int64) ([]entity.LeadSkResend, error) {
	if len(ids) == 0 {
		return []entity.LeadSkResend{}, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT lr.id, lr.uuid, COALESCE(d.skorozvon_project_id, '')
FROM lead_row lr
LEFT JOIN LATERAL (
	SELECT skorozvon_project_id
	FROM dispatch
	WHERE lead_row_id = lr.id AND channel_id = $2
	ORDER BY sent_at DESC
	LIMIT 1
) d ON true
WHERE lr.id = ANY($1)
`, ids, entity.ChannelSkorozvon)
	if err != nil {
		return nil, fmt.Errorf("list skorozvon resend: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadSkResend, 0, len(ids))
	for rows.Next() {
		var item entity.LeadSkResend
		if err := rows.Scan(&item.ID, &item.UUID, &item.ProjectID); err != nil {
			return nil, fmt.Errorf("scan skorozvon resend: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
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

func (r *LeadRepo) CountOutgoingDispatch(ctx context.Context, from, to time.Time) (entity.LeadOutgoingStats, error) {
	out := entity.LeadOutgoingStats{
		Partners:  []entity.LeadPartnerDispatchCount{},
		Skorozvon: []entity.LeadSkorozvonDispatchCount{},
		Crmcalls:  []entity.LeadSkorozvonDispatchCount{},
	}
	partnerRows, err := r.pool.Query(ctx, `
		SELECT upk_id, COUNT(*)::int
		FROM dispatch
		WHERE channel_id = $3
		  AND upk_id IS NOT NULL
		  AND sent_ok
		  AND sent_at >= $1 AND sent_at <= $2
		GROUP BY upk_id
	`, from, to, entity.ChannelPartner)
	if err != nil {
		return entity.LeadOutgoingStats{}, fmt.Errorf("count partner dispatch: %w", err)
	}
	defer partnerRows.Close()
	for partnerRows.Next() {
		var item entity.LeadPartnerDispatchCount
		if err := partnerRows.Scan(&item.PartnerID, &item.Count); err != nil {
			return entity.LeadOutgoingStats{}, fmt.Errorf("scan partner dispatch: %w", err)
		}
		out.Partners = append(out.Partners, item)
	}
	if err := partnerRows.Err(); err != nil {
		return entity.LeadOutgoingStats{}, err
	}

	skRows, err := r.pool.Query(ctx, `
		SELECT skorozvon_project_id, COUNT(*)::int
		FROM dispatch
		WHERE channel_id = $3
		  AND skorozvon_project_id IS NOT NULL
		  AND btrim(skorozvon_project_id) <> ''
		  AND sent_ok
		  AND sent_at >= $1 AND sent_at <= $2
		GROUP BY skorozvon_project_id
	`, from, to, entity.ChannelSkorozvon)
	if err != nil {
		return entity.LeadOutgoingStats{}, fmt.Errorf("count skorozvon dispatch: %w", err)
	}
	defer skRows.Close()
	for skRows.Next() {
		var item entity.LeadSkorozvonDispatchCount
		if err := skRows.Scan(&item.ProjectID, &item.Count); err != nil {
			return entity.LeadOutgoingStats{}, fmt.Errorf("scan skorozvon dispatch: %w", err)
		}
		out.Skorozvon = append(out.Skorozvon, item)
	}
	if err := skRows.Err(); err != nil {
		return entity.LeadOutgoingStats{}, err
	}

	ccRows, err := r.pool.Query(ctx, `
		SELECT skorozvon_project_id, COUNT(*)::int
		FROM dispatch
		WHERE channel_id = $3
		  AND skorozvon_project_id IS NOT NULL
		  AND btrim(skorozvon_project_id) <> ''
		  AND sent_ok
		  AND sent_at >= $1 AND sent_at <= $2
		GROUP BY skorozvon_project_id
	`, from, to, entity.ChannelCrmcalls)
	if err != nil {
		return entity.LeadOutgoingStats{}, fmt.Errorf("count crmcalls dispatch: %w", err)
	}
	defer ccRows.Close()
	for ccRows.Next() {
		var item entity.LeadSkorozvonDispatchCount
		if err := ccRows.Scan(&item.ProjectID, &item.Count); err != nil {
			return entity.LeadOutgoingStats{}, fmt.Errorf("scan crmcalls dispatch: %w", err)
		}
		out.Crmcalls = append(out.Crmcalls, item)
	}
	return out, ccRows.Err()
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
	COALESCE(zo.name, ''),
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
LEFT JOIN LATERAL (
	SELECT dv.name
	FROM dedup_result dr
	JOIN dedup_value dv ON dv.id = dr.value_id
	WHERE dr.lead_row_id = lr.id
	  AND dr.provider_id IN (SELECT p.id FROM dedup_provider p WHERE lower(p.name) IN ('заря старая', 'zarya_old'))
	ORDER BY dr.checked_at DESC
	LIMIT 1
) zo ON true
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

func (r *LeadRepo) GetForSkorozvon(ctx context.Context, uuid string) (entity.LeadForSkorozvon, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return entity.LeadForSkorozvon{}, fmt.Errorf("%w: uuid is required", domain.ErrInvalidInput)
	}
	var out entity.LeadForSkorozvon
	err := r.pool.QueryRow(ctx, `
		SELECT
			lr.id,
			lr.uuid,
			COALESCE(lr.phone, ''),
			lr.child_source_id,
			COALESCE(reg.name, ''),
			lr.region_id,
			COALESCE(z.name, ''),
			COALESCE(NULLIF(lr.cold_label, ''), cs.cold_label, ps.cold_label, ''),
			COALESCE(lr.source_description, '')
		FROM lead_row lr
		LEFT JOIN region reg ON reg.id = lr.region_id
		LEFT JOIN source ps ON ps.id = lr.source_id
		LEFT JOIN source cs ON cs.id = lr.child_source_id
		LEFT JOIN LATERAL (
			SELECT dv.name
			FROM dedup_result dr
			JOIN dedup_value dv ON dv.id = dr.value_id
			WHERE dr.lead_row_id = lr.id
			  AND dr.provider_id IN (SELECT p.id FROM dedup_provider p WHERE lower(p.name) IN ('заря', 'zarya'))
			ORDER BY dr.checked_at DESC
			LIMIT 1
		) z ON true
		WHERE lr.uuid = $1
	`, uuid).Scan(
		&out.ID,
		&out.UUID,
		&out.Phone,
		&out.ChildSourceID,
		&out.Region,
		&out.RegionID,
		&out.Zarya,
		&out.ColdLabel,
		&out.SourceDescription,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.LeadForSkorozvon{}, fmt.Errorf("%w: lead %s", domain.ErrNotFound, uuid)
		}
		return entity.LeadForSkorozvon{}, fmt.Errorf("get lead for skorozvon: %w", err)
	}
	return out, nil
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
		&d.ZaryaOld,
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

	queues, err := r.listOutboxQueues(ctx, d.UUID)
	if err != nil {
		return entity.LeadDetail{}, err
	}
	d.Queues = queues

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
			COALESCE(NULLIF(TRIM(f.value_text), ''), mo.name, '')
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

func (r *LeadRepo) listOutboxQueues(ctx context.Context, leadUUID string) ([]entity.LeadOutboxQueue, error) {
	leadUUID = strings.TrimSpace(leadUUID)
	if leadUUID == "" {
		return []entity.LeadOutboxQueue{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT uuid, 1::smallint, 'Скорозвон', count_retry, last_retry
		FROM outbox_skorozvon
		WHERE uuid_lead = $1
		UNION ALL
		SELECT uuid, 3::smallint, 'Битрикс', count_retry, last_retry
		FROM outbox_bx24
		WHERE uuid_lead = $1
	`, leadUUID)
	if err != nil {
		return nil, fmt.Errorf("list outbox queues: %w", err)
	}
	defer rows.Close()

	out := make([]entity.LeadOutboxQueue, 0)
	for rows.Next() {
		var item entity.LeadOutboxQueue
		if err := rows.Scan(&item.UUID, &item.ChannelID, &item.Channel, &item.CountRetry, &item.LastRetry); err != nil {
			return nil, fmt.Errorf("scan outbox queue: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows outbox queues: %w", err)
	}
	return out, nil
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

func leadListSkSelect(f port.LeadListFilter) string {
	if len(f.SkSend) == 0 {
		return `NULL::boolean, NULL::integer, ''::text`
	}
	return `sk.sent_ok, sk.http_status, COALESCE(sk.raw_response, '')`
}

func leadListSkJoin(f port.LeadListFilter) string {
	if len(f.SkSend) == 0 {
		return ""
	}
	return fmt.Sprintf(`
LEFT JOIN LATERAL (
	SELECT d.sent_ok, d.http_status, LEFT(COALESCE(d.raw_response, ''), 500) AS raw_response
	FROM dispatch d
	WHERE d.lead_row_id = lr.id AND d.channel_id = %d
	ORDER BY d.sent_at DESC
	LIMIT 1
) sk ON true`, entity.ChannelSkorozvon)
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
	if len(f.SkSend) > 0 {
		p := next(f.SkSend)
		b.WriteString(fmt.Sprintf(` AND (
	CASE
		WHEN NOT EXISTS (
			SELECT 1 FROM dispatch d
			WHERE d.lead_row_id = lr.id AND d.channel_id = %d
		) THEN 'none'
		WHEN (
			SELECT d.sent_ok
			FROM dispatch d
			WHERE d.lead_row_id = lr.id AND d.channel_id = %d
			ORDER BY d.sent_at DESC
			LIMIT 1
		) THEN 'ok'
		ELSE 'error'
	END
) = ANY(`, entity.ChannelSkorozvon, entity.ChannelSkorozvon) + p + `::text[])`)
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
