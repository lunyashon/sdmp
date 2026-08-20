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
	s3adapter "github.com/lunyashon/sdmp/internal/adapter/s3"
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

	sourceRepo := postgres.NewSourceRepo(pool)
	sourceSvc := usecase.NewSourceService(sourceRepo)
	leadSvc := usecase.NewLeadService(storage, sourceRepo, log)

	router := httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(leadSvc, log),
		Sources: handler.NewSourcesHandler(sourceSvc, log),
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
