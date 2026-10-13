package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"dockrunner_backend/internal/config"
	"dockrunner_backend/internal/consumer"
	"dockrunner_backend/internal/modules/admin"
	"dockrunner_backend/internal/modules/agents"
	"dockrunner_backend/internal/modules/balances"
	"dockrunner_backend/internal/modules/containers"
	"dockrunner_backend/internal/modules/deploymentrequests"
	"dockrunner_backend/internal/modules/pricing"
	"dockrunner_backend/internal/modules/users"
	"dockrunner_backend/internal/modules/volumes"
	"dockrunner_backend/internal/oplock"
	"dockrunner_backend/internal/router"
	"dockrunner_backend/pkg/database/influxdb"
	"dockrunner_backend/pkg/database/postgre"
	"dockrunner_backend/pkg/logger"
	"dockrunner_backend/pkg/queue"
)

// @title Dockrunner API
// @version 0.1.0
// @description Центральный API Dockrunner: пользователи, серверы, агенты, деплои из Git.
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer <access_token>. Токен выдаёт POST /api/v1/auth/login
func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	zlog, err := logger.NewWithConfig(logger.Config{Env: cfg.Env, Dir: cfg.LogDir})
	if err != nil {
		panic(fmt.Sprintf("failed to init logger: %v", err))
	}
	defer func() { _ = zlog.Sync() }()

	// Важное дублируется в терминал, остальное — только в файл.
	important := logger.Important(zlog)

	important.Info("starting dockrunner backend",
		zap.String("env", cfg.Env),
		zap.String("port", cfg.Port),
		zap.Bool("env_file", cfg.EnvFile),
		zap.String("postgres", config.RedactURL(cfg.PostgresURL)),
		zap.String("influxdb", config.RedactURL(cfg.InfluxURL)),
		zap.String("rabbitmq", config.RedactURL(cfg.RabbitURL)),
		zap.String("log_dir", cfg.LogDir),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pg := connectPostgres(ctx, cfg, zlog)
	if pg != nil {
		defer pg.Close()
	}

	influxDB := connectInflux(ctx, cfg, zlog)
	if influxDB != nil {
		defer influxDB.Close()
	}

	ops := oplock.New()
	queueClient := connectQueue(ctx, cfg, zlog)
	if queueClient != nil {
		defer func() { _ = queueClient.Close() }()
	}

	shared := buildSharedServices(ctx, cfg, pg, queueClient, ops, zlog, important)

	engine := router.New(router.Dependencies{
		Config:            cfg,
		Logger:            zlog,
		Postgres:          pg,
		Influx:            influxDB,
		Queue:             queueClient,
		OpTracker:         ops,
		AdminService:              shared.admin,
		VolumesService:            shared.volumes,
		ContainersService:         shared.containers,
		DeploymentRequestsService: shared.deploymentRequests,
		BalancesService:           shared.balances,
		PricingService:            shared.pricing,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      engine,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go serveHTTP(srv, important)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	important.Info("shutdown signal received", zap.String("signal", sig.String()))

	shutdownHTTP(srv, zlog)
	important.Info("server exited")
}

// connectPostgres открывает пул, накатывает миграции и сидит админа.
// Без базы возвращает nil — сервер работает в dev-режиме без postgres.
func connectPostgres(ctx context.Context, cfg *config.Config, zlog *zap.Logger) *pgxpool.Pool {
	pg, err := postgre.New(ctx, postgre.Config{
		URL:             cfg.PostgresURL,
		MaxConns:        cfg.PostgresMaxConns,
		MinConns:        cfg.PostgresMinConns,
		MaxConnLifetime: cfg.PostgresMaxConnLife,
		MaxConnIdle:     cfg.PostgresMaxConnIdle,
		HealthCheck:     cfg.PostgresHealthCheck,
	}, zlog)
	if err != nil {
		zlog.Warn("postgres not available (dev mode continuing)", zap.Error(err))
		return nil
	}

	_ = postgre.Migrate(ctx, pg, zlog)
	users.SeedAdmin(ctx, pg, cfg.AdminLogin, cfg.AdminPassword, cfg.BcryptCost, zlog)
	return pg
}

// connectInflux подключает InfluxDB для метрик. Без URL или при ошибке — nil.
func connectInflux(ctx context.Context, cfg *config.Config, zlog *zap.Logger) *influxdb.Client {
	if cfg.InfluxURL == "" {
		return nil
	}

	influxClient, err := influxdb.New(ctx, influxdb.Config{
		URL:     cfg.InfluxURL,
		Token:   cfg.InfluxToken,
		Org:     cfg.InfluxOrg,
		Bucket:  cfg.InfluxBucket,
		Timeout: cfg.InfluxTimeout,
	}, zlog)
	if err != nil {
		zlog.Warn("influxdb not available", zap.Error(err))
		return nil
	}

	return influxClient
}

// connectQueue открывает RabbitMQ и объявляет топологию. При ошибке — nil.
func connectQueue(ctx context.Context, cfg *config.Config, zlog *zap.Logger) *queue.Client {
	queueClient, err := queue.Dial(ctx, queue.Config{URL: cfg.RabbitURL, TLSSkipVerify: cfg.RabbitTLSSkipVerify}, zlog)
	if err != nil {
		zlog.Warn("rabbitmq not available (dev mode continuing)", zap.Error(err))
		return nil
	}

	if err := queueClient.DeclareTopology(ctx); err != nil {
		zlog.Warn("rabbitmq topology failed", zap.Error(err))
	}

	return queueClient
}

type sharedServices struct {
	admin              *admin.Service
	volumes            *volumes.Service
	containers         *containers.Service
	deploymentRequests *deploymentrequests.Service
	balances           *balances.Service
	pricing            *pricing.Service
}

// buildSharedServices собирает один набор сервисов для HTTP-роутера и consumer'а
// событий (у containers in-memory проверки — два инстанса разрешали бы результаты
// в карту, которую никто не опрашивает).
// Без postgres+rabbitmq возвращает пустой набор — роутер поднимет локальные
// инстансы (dev-режим без результатов задач).
func buildSharedServices(ctx context.Context, cfg *config.Config, pg *pgxpool.Pool, queueClient *queue.Client, ops *oplock.Tracker, zlog *zap.Logger, important *zap.Logger) sharedServices {
	if queueClient == nil {
		return sharedServices{}
	}
	if pg == nil {
		zlog.Warn("events consumer skipped — no postgres, task results will not arrive")
		return sharedServices{}
	}

	adminRepo := admin.NewRepository(pg, zlog)
	adminService := admin.NewService(adminRepo, queueClient, zlog, ops)

	agentsRepo := agents.NewRepository(pg, zlog)

	volumesRepo := volumes.NewRepository(pg, zlog)
	volumesService := volumes.NewService(volumesRepo, queueClient, zlog)

	containersService := containers.NewService(adminRepo, agentsRepo, volumesService, cfg.AgentToken, queueClient, zlog, ops, containers.PortsPolicy{Min: cfg.PortsMin, Max: cfg.PortsMax})

	// Deployment Requests
	deploymentRequestsRepo := deploymentrequests.NewRepository(pg, zlog)

	// Pricing
	pricingRepo := pricing.NewRepository(pg, zlog)
	pricingService := pricing.NewService(pricingRepo, zlog)

	// Balances
	balancesRepo := balances.NewRepository(pg, zlog)
	balancesService := balances.NewService(balancesRepo, zlog)

	// Deployment Requests service (needs pricing, containers to launch, balances to charge)
	deploymentRequestsService := deploymentrequests.NewService(deploymentRequestsRepo, pricingRepo, containersService, balancesService, zlog)

	if err := consumer.StartEvents(ctx, queueClient, adminService, containersService, zlog); err != nil {
		zlog.Warn("events consumer failed", zap.Error(err))
		return sharedServices{}
	}

	important.Info("events consumer started")
	return sharedServices{
		admin:              adminService,
		volumes:            volumesService,
		containers:         containersService,
		deploymentRequests: deploymentRequestsService,
		balances:           balancesService,
		pricing:            pricingService,
	}
}

// serveHTTP слушает порт. Падение — в терминал (Error) с выходом.
func serveHTTP(srv *http.Server, important *zap.Logger) {
	important.Info("http server listening", zap.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		important.Error("listen failed", zap.Error(err))
		os.Exit(1)
	}
}

// shutdownHTTP мягко останавливает сервер с таймаутом 10 секунд.
func shutdownHTTP(srv *http.Server, zlog *zap.Logger) {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		zlog.Error("server shutdown failed", zap.Error(err))
		if err := srv.Close(); err != nil {
			zlog.Error("server close failed", zap.Error(err))
		}
	}
}
