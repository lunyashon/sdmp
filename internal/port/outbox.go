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
