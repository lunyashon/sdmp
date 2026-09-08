package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/adapter/http/server"
	"github.com/lunyashon/sdmp/internal/adapter/postgres"
	redisadapter "github.com/lunyashon/sdmp/internal/adapter/redis"
	s3adapter "github.com/lunyashon/sdmp/internal/adapter/s3"
	"github.com/lunyashon/sdmp/internal/adapter/skorozvon"
	"github.com/lunyashon/sdmp/internal/lib/config"
	sl "github.com/lunyashon/sdmp/internal/lib/logger"
	"github.com/lunyashon/sdmp/internal/usecase"
)

func Run() error {
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

	storage, err := s3adapter.New(ctx, s3adapter.Config{
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Endpoint:  cfg.S3Endpoint,
		Region:    cfg.S3Region,
		Bucket:    cfg.S3Bucket,
	}, log)
	if err != nil {
		return fmt.Errorf("init s3: %w", err)
	}
	if err := storage.Ping(ctx); err != nil {
		return fmt.Errorf("s3 ping: %w", err)
	}
	log.Info("s3 ready", "bucket", cfg.S3Bucket, "endpoint", cfg.S3Endpoint)

	cache, err := redisadapter.New(redisadapter.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}, log)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	defer cache.Close()
	if err := cache.Ping(ctx); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	log.Info("redis ready", "addr", cfg.RedisAddr)

	sourceRepo := postgres.NewSourceRepo(pool)
	leadRepo := postgres.NewLeadRepo(pool)
	dictRepo := postgres.NewDictRepo(pool)
	partnerRepo := postgres.NewPartnerRepo(pool)
	splitRepo := postgres.NewSplitRepo(pool)
	skorozvonProjectRepo := postgres.NewSkorozvonProjectRepo(pool)
	sourceSvc := usecase.NewSourceService(sourceRepo)
	partnerPoster := usecase.NewPartnerPoster(8 * time.Second)
	leadSvc := usecase.NewLeadService(leadRepo, log).
		WithStorage(storage).
		WithPartners(partnerRepo, partnerPoster).
		WithDicts(dictRepo).
		WithSkorozvonMaps(postgres.NewSkorozvonMapRepo(pool))
	partnerSvc := usecase.NewPartnerService(partnerRepo, partnerPoster)
	splitSvc := usecase.NewSplitService(splitRepo, sourceRepo, dictRepo)
	skorozvonClient := skorozvon.New(skorozvon.Config{
		Login:  cfg.SkorozvonLogin,
		APIKey: cfg.SkorozvonAPIKey,
		AppID:  cfg.SkorozvonAppID,
		AppKey: cfg.SkorozvonAppKey,
	}, cache, log)
	skorozvonProjectSvc := usecase.NewSkorozvonProjectService(skorozvonProjectRepo, skorozvonClient)
	skorozvonLinkSvc := usecase.NewSkorozvonLinkService(postgres.NewSkorozvonLinkRepo(pool))
	receiveSvc := usecase.NewReceiveLeadService(sourceRepo, leadRepo, storage, cache, log)

	router := httpgin.NewRouter(httpgin.Handlers{
		Leads:             handler.NewLeadsHandler(leadSvc, log),
		Sources:           handler.NewSourcesHandler(sourceSvc, log),
		Dicts:             handler.NewDictsHandler(dictRepo, log),
		Partners:          handler.NewPartnersHandler(partnerSvc, log),
		Splits:            handler.NewSplitsHandler(splitSvc, log),
		SkorozvonProjects: handler.NewSkorozvonProjectsHandler(skorozvonProjectSvc, log),
		SkorozvonLinks:    handler.NewSkorozvonLinksHandler(skorozvonLinkSvc, log),
		SkorozvonEvents:   handler.NewSkorozvonEventsHandler(leadSvc, log),
		CRMLead:           handler.NewCRMLeadHandler(receiveSvc, log),
	})
	srv := server.New(cfg.Port, router, log, cfg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		return nil
	}
}

func configPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return "config/.env"
}
