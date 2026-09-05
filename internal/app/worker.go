package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/lunyashon/sdmp/internal/adapter/postgres"
	redisadapter "github.com/lunyashon/sdmp/internal/adapter/redis"
	"github.com/lunyashon/sdmp/internal/adapter/skorozvon"
	"github.com/lunyashon/sdmp/internal/lib/config"
	sl "github.com/lunyashon/sdmp/internal/lib/logger"
	"github.com/lunyashon/sdmp/internal/lib/phonesearch"
	"github.com/lunyashon/sdmp/internal/usecase"
)

func RunWorker() error {
	cfg, err := config.Load(configPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := sl.ExecLog(
		cfg.LogOutputPath,
		cfg.LogLevel,
		cfg.LogOutputMaxSize,
		cfg.LogOutputMaxAge,
		cfg.LogOutputMaxBackups,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := postgres.NewPool(ctx, postgres.DSN(
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
	))
	if err != nil {
		return fmt.Errorf("init postgres: %w", err)
	}
	defer pool.Close()

	cache, err := redisadapter.New(redisadapter.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}, log)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	defer cache.Close()

	phoneSearch := phonesearch.New(cfg.PhoneSearchURL, cfg.PhoneSearchToken, cfg.ZaryaURL, http.DefaultClient)
	skorozvonClient := skorozvon.New(skorozvon.Config{
		Login:  cfg.SkorozvonLogin,
		APIKey: cfg.SkorozvonAPIKey,
		AppID:  cfg.SkorozvonAppID,
		AppKey: cfg.SkorozvonAppKey,
	}, cache, log)

	runtime.GOMAXPROCS(cfg.Gomaxprocs)
	worker := usecase.NewWorkerService(
		postgres.NewOutboxRepo(pool),
		phoneSearch,
		postgres.NewDictRepo(pool),
		postgres.NewLeadRepo(pool),
		postgres.NewSplitRepo(pool),
		postgres.NewPartnerRepo(pool),
		log,
		cfg.Gomaxprocs,
	).WithSkorozvon(postgres.NewSkorozvonLinkRepo(pool), skorozvonClient)

	log.Info("worker started", "workers", cfg.Gomaxprocs)
	if err := worker.Run(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	log.Info("worker stopped")
	return nil
}
