package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/id"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.OutboxSkorozvonRepository = (*OutboxSkorozvonRepo)(nil)

const outboxSkorozvonAutoRetries = 6

type OutboxSkorozvonRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxSkorozvonRepo(pool *pgxpool.Pool) *OutboxSkorozvonRepo {
	return &OutboxSkorozvonRepo{pool: pool}
}

func (r *OutboxSkorozvonRepo) Enqueue(ctx context.Context, leadUUID string) error {
	leadUUID = strings.TrimSpace(leadUUID)
	if leadUUID == "" {
		return fmt.Errorf("enqueue outbox_skorozvon: uuid_lead is required")
	}
	outboxID, err := id.NewUUID()
	if err != nil {
		return fmt.Errorf("outbox_skorozvon id: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO outbox_skorozvon (uuid, uuid_lead, locked_until, count_retry)
		VALUES ($1, $2, NULL, 0)
		ON CONFLICT (uuid_lead) DO NOTHING
	`, outboxID, leadUUID)
	if err != nil {
		return fmt.Errorf("enqueue outbox_skorozvon: %w", err)
	}
	return nil
}

func (r *OutboxSkorozvonRepo) EnqueueResend(ctx context.Context, leadUUID string, projectID *int64) error {
	leadUUID = strings.TrimSpace(leadUUID)
	if leadUUID == "" {
		return fmt.Errorf("enqueue outbox_skorozvon: uuid_lead is required")
	}
	outboxID, err := id.NewUUID()
	if err != nil {
		return fmt.Errorf("outbox_skorozvon id: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO outbox_skorozvon (uuid, uuid_lead, project_id, locked_until, count_retry)
		VALUES ($1, $2, $3, NULL, 0)
		ON CONFLICT (uuid_lead) DO UPDATE SET
			count_retry = 0,
			locked_until = NULL,
			last_retry = NULL,
			project_id = COALESCE(EXCLUDED.project_id, outbox_skorozvon.project_id)
	`, outboxID, leadUUID, projectID)
	if err != nil {
		return fmt.Errorf("enqueue outbox_skorozvon resend: %w", err)
	}
	return nil
}

func (r *OutboxSkorozvonRepo) Claim(ctx context.Context, limit int, lockFor time.Duration) ([]entity.OutboxSkorozvon, error) {
	if limit <= 0 {
		limit = 1
	}
	if lockFor <= 0 {
		lockFor = 2 * time.Minute
	}

	rows, err := r.pool.Query(ctx, `
		WITH picked AS (
			SELECT uuid
			FROM outbox_skorozvon
			WHERE count_retry < $3
			  AND (locked_until IS NULL OR locked_until <= now())
			ORDER BY uuid
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE outbox_skorozvon o
		SET locked_until = now() + ($2 * interval '1 second')
		FROM picked
		WHERE o.uuid = picked.uuid
		RETURNING o.uuid, o.uuid_lead, o.project_id, o.locked_until, o.count_retry, o.last_retry
	`, limit, int64(lockFor.Seconds()), outboxSkorozvonAutoRetries)
	if err != nil {
		return nil, fmt.Errorf("claim outbox_skorozvon: %w", err)
	}
	defer rows.Close()

	out := make([]entity.OutboxSkorozvon, 0, limit)
	for rows.Next() {
		var item entity.OutboxSkorozvon
		if err := rows.Scan(&item.UUID, &item.LeadUUID, &item.ProjectID, &item.LockedUntil, &item.CountRetry, &item.LastRetry); err != nil {
			return nil, fmt.Errorf("scan outbox_skorozvon: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows outbox_skorozvon: %w", err)
	}
	return out, nil
}

func (r *OutboxSkorozvonRepo) MarkRetry(ctx context.Context, uuid string, countRetry int16, lockFor time.Duration) error {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return fmt.Errorf("retry outbox_skorozvon: uuid is required")
	}
	var err error
	if lockFor < 0 {
		_, err = r.pool.Exec(ctx, `
			UPDATE outbox_skorozvon
			SET count_retry = $2,
			    last_retry = now(),
			    locked_until = NULL
			WHERE uuid = $1
		`, uuid, countRetry)
	} else {
		_, err = r.pool.Exec(ctx, `
			UPDATE outbox_skorozvon
			SET count_retry = $2,
			    last_retry = now(),
			    locked_until = now() + ($3 * interval '1 second')
			WHERE uuid = $1
		`, uuid, countRetry, int64(lockFor.Seconds()))
	}
	if err != nil {
		return fmt.Errorf("retry outbox_skorozvon: %w", err)
	}
	return nil
}

func (r *OutboxSkorozvonRepo) Complete(ctx context.Context, uuid string) error {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return fmt.Errorf("complete outbox_skorozvon: uuid is required")
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM outbox_skorozvon WHERE uuid = $1`, uuid)
	if err != nil {
		return fmt.Errorf("complete outbox_skorozvon: %w", err)
	}
	return nil
}
