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

var _ port.OutboxCrmcallsRepository = (*OutboxCrmcallsRepo)(nil)

const outboxCrmcallsAutoRetries = 6

type OutboxCrmcallsRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxCrmcallsRepo(pool *pgxpool.Pool) *OutboxCrmcallsRepo {
	return &OutboxCrmcallsRepo{pool: pool}
}

func (r *OutboxCrmcallsRepo) Enqueue(ctx context.Context, leadUUID string) error {
	leadUUID = strings.TrimSpace(leadUUID)
	if leadUUID == "" {
		return fmt.Errorf("enqueue outbox_crmcalls: uuid_lead is required")
	}
	outboxID, err := id.NewUUID()
	if err != nil {
		return fmt.Errorf("outbox_crmcalls id: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO outbox_crmcalls (uuid, uuid_lead, locked_until, count_retry)
		VALUES ($1, $2, NULL, 0)
		ON CONFLICT (uuid_lead) DO NOTHING
	`, outboxID, leadUUID)
	if err != nil {
		return fmt.Errorf("enqueue outbox_crmcalls: %w", err)
	}
	return nil
}

func (r *OutboxCrmcallsRepo) Claim(ctx context.Context, limit int, lockFor time.Duration) ([]entity.OutboxCrmcalls, error) {
	if limit <= 0 {
		limit = 1
	}
	if lockFor <= 0 {
		lockFor = 2 * time.Minute
	}

	rows, err := r.pool.Query(ctx, `
		WITH picked AS (
			SELECT uuid
			FROM outbox_crmcalls
			WHERE count_retry < $3
			  AND (locked_until IS NULL OR locked_until <= now())
			ORDER BY uuid
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE outbox_crmcalls o
		SET locked_until = now() + ($2 * interval '1 second')
		FROM picked
		WHERE o.uuid = picked.uuid
		RETURNING o.uuid, o.uuid_lead, o.locked_until, o.count_retry, o.last_retry
	`, limit, int64(lockFor.Seconds()), outboxCrmcallsAutoRetries)
	if err != nil {
		return nil, fmt.Errorf("claim outbox_crmcalls: %w", err)
	}
	defer rows.Close()

	out := make([]entity.OutboxCrmcalls, 0, limit)
	for rows.Next() {
		var item entity.OutboxCrmcalls
		if err := rows.Scan(&item.UUID, &item.LeadUUID, &item.LockedUntil, &item.CountRetry, &item.LastRetry); err != nil {
			return nil, fmt.Errorf("scan outbox_crmcalls: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *OutboxCrmcallsRepo) MarkRetry(ctx context.Context, uuid string, countRetry int16, lockFor time.Duration) error {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return fmt.Errorf("retry outbox_crmcalls: uuid is required")
	}
	var err error
	if lockFor < 0 {
		_, err = r.pool.Exec(ctx, `
			UPDATE outbox_crmcalls
			SET count_retry = $2, last_retry = now(), locked_until = NULL
			WHERE uuid = $1
		`, uuid, countRetry)
	} else {
		_, err = r.pool.Exec(ctx, `
			UPDATE outbox_crmcalls
			SET count_retry = $2,
			    last_retry = now(),
			    locked_until = now() + ($3 * interval '1 second')
			WHERE uuid = $1
		`, uuid, countRetry, int64(lockFor.Seconds()))
	}
	if err != nil {
		return fmt.Errorf("retry outbox_crmcalls: %w", err)
	}
	return nil
}

func (r *OutboxCrmcallsRepo) Complete(ctx context.Context, uuid string) error {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return fmt.Errorf("complete outbox_crmcalls: uuid is required")
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM outbox_crmcalls WHERE uuid = $1`, uuid)
	if err != nil {
		return fmt.Errorf("complete outbox_crmcalls: %w", err)
	}
	return nil
}
