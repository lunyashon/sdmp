package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/lunyashon/sdmp/internal/adapter/crmcalls"
	"github.com/lunyashon/sdmp/internal/adapter/google"
	"github.com/lunyashon/sdmp/internal/adapter/postgres"
	redisadapter "github.com/lunyashon/sdmp/internal/adapter/redis"
	"github.com/lunyashon/sdmp/internal/adapter/skorozvon"
	"github.com/lunyashon/sdmp/internal/lib/config"
	"github.com/lunyashon/sdmp/internal/lib/crm"
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
		cfg.DBMaxConns,
	))
	if err != nil {
		return fmt.Errorf("init postgres: %w", err)
	}
	defer pool.Close()
	log.Info("postgres ready", "max_conns", cfg.DBMaxConns)

	cache, err := redisadapter.New(redisadapter.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}, log)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	defer cache.Close()

	phoneSearch := phonesearch.New(cfg.PhoneSearchURL, cfg.PhoneSearchToken, cfg.ZaryaURL, http.DefaultClient).
		WithZaryaBitrix(cfg.ZaryaBitrixURL).
		WithForceClean(cfg.ZaryaForceClean)
	if cfg.ZaryaForceClean {
		log.Warn("zarya force clean: crm-zennit.ru duplicate checks skipped")
	}
	skorozvonClient := skorozvon.New(skorozvon.Config{
		Login:  cfg.SkorozvonLogin,
		APIKey: cfg.SkorozvonAPIKey,
		AppID:  cfg.SkorozvonAppID,
		AppKey: cfg.SkorozvonAppKey,
		RPS:    cfg.SkorozvonRPS,
	}, cache, log)

	runtime.GOMAXPROCS(cfg.Gomaxprocs)
	skorozvonProjectSvc := usecase.NewSkorozvonProjectService(postgres.NewSkorozvonProjectRepo(pool), skorozvonClient)
	worker := usecase.NewWorkerService(
		postgres.NewOutboxRepo(pool),
		phoneSearch,
		postgres.NewDictRepo(pool),
		postgres.NewLeadRepo(pool),
		postgres.NewSplitRepo(pool),
		postgres.NewPartnerRepo(pool),
		log,
		cfg.Gomaxprocs,
	).WithSkorozvon(postgres.NewSkorozvonLinkRepo(pool), skorozvonClient).
		WithSkorozvonMaps(postgres.NewSkorozvonMapRepo(pool)).
		WithBitrixMaps(postgres.NewBitrixMapRepo(pool)).
		WithSkorozvonProjects(skorozvonProjectSvc).
		WithOutboxBx24(postgres.NewOutboxBx24Repo(pool)).
		WithOutboxSkorozvon(postgres.NewOutboxSkorozvonRepo(pool)).
		WithBx24(crm.New(cfg.SodatechBx24URL, http.DefaultClient)).
		WithRecordingLink(cfg.PublicBaseURL, cfg.RecordingLinkSecret).
		WithGoogleSheets(postgres.NewGoogleSheetRepo(pool), google.New()).
		WithCrmcalls(
			postgres.NewCrmcallsLinkRepo(pool),
			crmcalls.NewDynamic(crmcalls.Config{
				BaseURL: cfg.CrmcallsBaseURL,
				APIKey:  cfg.CrmcallsAPIKey,
				RPS:     cfg.CrmcallsRPS,
			}, postgres.NewCrmcallsSettingsRepo(pool)),
			postgres.NewOutboxCrmcallsRepo(pool),
		)

	log.Info("worker started", "workers", cfg.Gomaxprocs, "skorozvon_rps", cfg.SkorozvonRPS)
	if err := worker.Run(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	log.Info("worker stopped")
	return nil
}
