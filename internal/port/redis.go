package port

import (
	"context"
	"time"
)

type Redis interface {
	Ping(ctx context.Context) error
	HSet(ctx context.Context, key string, values map[string]string) error
	HGet(ctx context.Context, key, field string) (string, bool, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Close() error
}
