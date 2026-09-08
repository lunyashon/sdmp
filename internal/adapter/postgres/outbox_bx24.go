package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.OutboxBx24Repository = (*OutboxBx24Repo)(nil)

type OutboxBx24Repo struct {
	pool *pgxpool.Pool
}

func NewOutboxBx24Repo(pool *pgxpool.Pool) *OutboxBx24Repo {
	return &OutboxBx24Repo{pool: pool}
}

func (r *OutboxBx24Repo) Claim(ctx context.Context, limit int, staleAfter time.Duration) ([]entity.OutboxBx24, error) {
	if limit <= 0 {
		limit = 1
	}
	if staleAfter <= 0 {
		staleAfter = 5 * time.Minute
	}

	rows, err := r.pool.Query(ctx, `
		WITH picked AS (
			SELECT uuid
			FROM outbox_bx24
			WHERE locked_until IS NULL
			   OR locked_until < now() - ($2 * interval '1 second')
			ORDER BY uuid
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE outbox_bx24 o
		SET locked_until = now()
		FROM picked
		WHERE o.uuid = picked.uuid
		RETURNING o.uuid, o.uuid_lead, o.locked_until
	`, limit, int64(staleAfter.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("claim outbox_bx24: %w", err)
	}
	defer rows.Close()

	out := make([]entity.OutboxBx24, 0, limit)
	for rows.Next() {
		var item entity.OutboxBx24
		if err := rows.Scan(&item.UUID, &item.LeadUUID, &item.LockedUntil); err != nil {
			return nil, fmt.Errorf("scan outbox_bx24: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows outbox_bx24: %w", err)
	}
	return out, nil
}

func (r *OutboxBx24Repo) Complete(ctx context.Context, uuid string) error {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return fmt.Errorf("complete outbox_bx24: uuid is required")
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM outbox_bx24 WHERE uuid = $1`, uuid)
	if err != nil {
		return fmt.Errorf("complete outbox_bx24: %w", err)
	}
	return nil
}
