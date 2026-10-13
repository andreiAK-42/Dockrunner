package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"dockrunner_agent/internal/api"
	"dockrunner_agent/internal/buildlog"
	"dockrunner_agent/internal/config"
	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/geo"
	"dockrunner_agent/internal/heartbeat"
	"dockrunner_agent/internal/logs"
	"dockrunner_agent/internal/queue"
	"dockrunner_agent/internal/store"
	"dockrunner_agent/internal/worker"
)

// @title Dockrunner Agent API
// @version 0.1.0
// @description HTTP API воркера: контейнеры, хвост логов, WebSocket-стрим. Задачи принимает по RabbitMQ.
// @BasePath /
func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("starting dockrunner agent",
		"version", config.Version,
		"agent", cfg.AgentID,
		"env_file", cfg.EnvFile,
		"rabbitmq", config.RedactURL(cfg.RabbitURL),
	)
	if !cfg.EnvFile {
		logger.Warn("no .env file found — using defaults and OS environment; run from the agent directory or beside the binary")
	}
	if cfg.AgentPublicURL == "" {
		logger.Warn("AGENT_PUBLIC_URL is not set — central cannot proxy container spec/logs and build logs; set it to this agent HTTP address")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.AgentLat == 0 && cfg.AgentLng == 0 {
		if loc := geo.Detect(ctx, logger); loc.Lat != 0 || loc.Lng != 0 {
			cfg.AgentCity, cfg.AgentLat, cfg.AgentLng = loc.City, loc.Lat, loc.Lng
			logger.Info("geo detected", "city", loc.City, "lat", loc.Lat, "lng", loc.Lng)
		}
	}

	db, err := store.Open(cfg.SQLitePath)
	if err != nil {
		logger.Error("open sqlite failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	dockCli, err := docker.New()
	if err != nil {
		logger.Error("docker unavailable", "error", err)
		os.Exit(1)
	}
	defer func() { _ = dockCli.Close() }()
	if _, err := dockCli.RunningIDs(ctx); err != nil {
		logger.Error("docker ping failed", "error", err)
		os.Exit(1)
	}

	queueClient := connectQueue(ctx, cfg, logger)
	if queueClient != nil {
		defer func() { _ = queueClient.Close() }()
	}

	streamer := logs.New(dockCli, logger)
	builds := buildlog.New(db)
	reattach(ctx, logger, db, dockCli, streamer)

	if queueClient != nil {
		handler := worker.NewHandler(cfg.AgentID, cfg.Workspace, db, dockCli, queueClient, logger, builds, streamer.Attach, streamer.Detach)
		if err := queueClient.Consume(queue.AgentQueue(cfg.AgentID), 4, handler.Handle); err != nil {
			logger.Error("consume tasks failed", "error", err)
			os.Exit(1)
		}
	}

	hostname, _ := os.Hostname()
	go heartbeat.Start(ctx, logger, cfg.CentralURL, cfg.Token, cfg.AgentID, hostname, cfg.AgentName, cfg.AgentDescription, cfg.AgentCity, cfg.AgentLat, cfg.AgentLng, cfg.AgentPublicURL, cfg.Heartbeat, db, dockCli, cfg.Workspace)

	apiSrv := api.New(cfg, db, dockCli, streamer, builds, logger.Info)
	httpSrv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      apiSrv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	go func() {
		logger.Info("agent api listening", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutdown signal received", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("api shutdown failed", "error", err)
	}
	streamer.Stop()
	logger.Info("agent exited")
}

// connectQueue подключает RabbitMQ. Без очереди возвращает nil —
// задачи и стрим логов недоступны, HTTP API продолжает работать.
func connectQueue(ctx context.Context, cfg config.Config, logger *slog.Logger) *queue.Client {
	queueClient, err := queue.Dial(ctx, cfg.RabbitURL, cfg.RabbitTLSSkipVerify, logger)
	if err != nil {
		logger.Warn("rabbitmq not available, tasks and log streaming disabled", "error", err)
		return nil
	}
	if err := queueClient.DeclareTopology(cfg.AgentID); err != nil {
		logger.Warn("rabbitmq topology failed", "error", err)
	}
	return queueClient
}

func reattach(ctx context.Context, logger *slog.Logger, db *sql.DB, dockCli *docker.Client, streamer *logs.Streamer) {
	deployments, err := store.ListDeployments(db)
	if err != nil {
		logger.Warn("list deployments failed", "error", err)
		return
	}
	for _, d := range deployments {
		if d.Status != "running" {
			continue
		}
		status, err := dockCli.ContainerStatus(ctx, d.Container)
		if err != nil || status != "running" {
			_ = store.UpdateDeploymentStatus(db, d.ID, "stopped", d.Container)
			continue
		}
		streamer.Attach(d.Container, d.AppName)
	}
}
