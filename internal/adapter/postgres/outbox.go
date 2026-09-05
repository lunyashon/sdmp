package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/id"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.OutboxRepository = (*OutboxRepo)(nil)

type OutboxRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxRepo(pool *pgxpool.Pool) *OutboxRepo {
	return &OutboxRepo{pool: pool}
}

func (r *OutboxRepo) Claim(ctx context.Context, limit int, lockFor time.Duration) (port.OutboxClaim, error) {
	if limit <= 0 {
		limit = 1
	}
	if lockFor <= 0 {
		lockFor = 2 * time.Minute
	}

	token, err := id.NewUUID()
	if err != nil {
		return port.OutboxClaim{}, fmt.Errorf("claim token: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return port.OutboxClaim{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT o.id, o.lead_row_id, o.source_id, o.payload, o.created_at, COALESCE(lr.uuid::text, '')
		FROM outbox o
		LEFT JOIN lead_row lr ON lr.id = o.lead_row_id
		WHERE o.locked_until IS NULL OR o.locked_until < now()
		ORDER BY o.id
		FOR UPDATE OF o SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		return port.OutboxClaim{}, fmt.Errorf("select: %w", err)
	}

	msgs := make([]entity.OutboxMessage, 0, limit)
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var msg entity.OutboxMessage
		if err := rows.Scan(&msg.ID, &msg.LeadRowID, &msg.SourceID, &msg.Payload, &msg.CreatedAt, &msg.LeadUUID); err != nil {
			rows.Close()
			return port.OutboxClaim{}, fmt.Errorf("scan: %w", err)
		}
		msgs = append(msgs, msg)
		ids = append(ids, msg.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return port.OutboxClaim{}, fmt.Errorf("rows: %w", err)
	}

	if len(msgs) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return port.OutboxClaim{}, fmt.Errorf("commit empty: %w", err)
		}
		return port.OutboxClaim{}, nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE outbox
		SET locked_until = now() + ($2 * interval '1 second'),
		    locked_by = $1::uuid
		WHERE id = ANY($3)
	`, token, int64(lockFor.Seconds()), ids)
	if err != nil {
		return port.OutboxClaim{}, fmt.Errorf("lock: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return port.OutboxClaim{}, fmt.Errorf("commit: %w", err)
	}
	return port.OutboxClaim{Token: token, Messages: msgs}, nil
}

func (r *OutboxRepo) Complete(ctx context.Context, token string, id int64) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM outbox
		WHERE id = $1 AND locked_by = $2::uuid
	`, id, token)
	if err != nil {
		return fmt.Errorf("complete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("complete: outbox %d lease lost", id)
	}
	return nil
}

func (r *OutboxRepo) Release(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox
		SET locked_until = NULL, locked_by = NULL
		WHERE locked_by = $1::uuid
	`, token)
	if err != nil {
		return fmt.Errorf("release: %w", err)
	}
	return nil
}
