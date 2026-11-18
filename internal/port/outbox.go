package port

import (
	"context"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type OutboxClaim struct {
	Token    string
	Messages []entity.OutboxMessage
}

type OutboxRepository interface {
	Claim(ctx context.Context, limit int, ttl time.Duration) (OutboxClaim, error)
	Complete(ctx context.Context, token string, id int64) error
	Release(ctx context.Context, token string) error
}

type OutboxBx24Repository interface {
	Claim(ctx context.Context, limit int, lockFor time.Duration) ([]entity.OutboxBx24, error)
	MarkRetry(ctx context.Context, uuid string, countRetry int16, lockFor time.Duration) error
	Complete(ctx context.Context, uuid string) error
}

type OutboxSkorozvonRepository interface {
	Enqueue(ctx context.Context, leadUUID string) error
	EnqueueResend(ctx context.Context, leadUUID string, projectID *int64) error
	Claim(ctx context.Context, limit int, lockFor time.Duration) ([]entity.OutboxSkorozvon, error)
	MarkRetry(ctx context.Context, uuid string, countRetry int16, lockFor time.Duration) error
	Complete(ctx context.Context, uuid string) error
}

type OutboxCrmcallsRepository interface {
	Enqueue(ctx context.Context, leadUUID string) error
	Claim(ctx context.Context, limit int, lockFor time.Duration) ([]entity.OutboxCrmcalls, error)
	MarkRetry(ctx context.Context, uuid string, countRetry int16, lockFor time.Duration) error
	Complete(ctx context.Context, uuid string) error
}
