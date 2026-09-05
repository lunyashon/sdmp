package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.Redis = (*Client)(nil)

type Config struct {
	Addr     string
	Password string
	DB       int
}

type Client struct {
	rdb *goredis.Client
	log *slog.Logger
}

func New(cfg Config, log *slog.Logger) (*Client, error) {
	if log == nil {
		log = slog.Default()
	}
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:6379"
	}
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{rdb: rdb, log: log}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) HSet(ctx context.Context, key string, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	fields := make([]any, 0, len(values)*2)
	for k, v := range values {
		fields = append(fields, k, v)
	}
	return c.rdb.HSet(ctx, key, fields...).Err()
}

func (c *Client) HGet(ctx context.Context, key, field string) (string, bool, error) {
	val, err := c.rdb.HGet(ctx, key, field).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.rdb.HGetAll(ctx, key).Result()
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *Client) Close() error {
	if err := c.rdb.Close(); err != nil {
		return fmt.Errorf("redis close: %w", err)
	}
	return nil
}
