package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}

func DSN(host, port, user, password, dbname string, maxConns int) string {
	if maxConns < 1 {
		maxConns = 32
	}
	minConns := 4
	if maxConns < minConns {
		minConns = maxConns
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Europe/Moscow pool_max_conns=%d pool_min_conns=%d",
		host, port, user, password, dbname, maxConns, minConns,
	)
}
