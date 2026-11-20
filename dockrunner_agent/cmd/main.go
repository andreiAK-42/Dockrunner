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
	"dockrunner_agent/internal/victoria"
	"dockrunner_agent/internal/worker"
)

// @title Dockrunner Agent API
// @version 0.1.0
// @description HTTP API воркера: контейнеры, хвост логов, WebSocket-стрим. Задачи принимает по RabbitMQ.
// @BasePath /
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Ошибка загрузки конфигурации", "error", err)
		os.Exit(1)
	}

	//Логирование
	logHandler := slog.Handler(slog.NewTextHandler(os.Stdout, nil))
	var vlogHandler *victoria.LogHandler
	
	vlogHandler = victoria.NewLogHandler(cfg.VictoriaLogsURL, cfg.VictoriaToken,
			"dockrunner-agent", "worker",  cfg.AgentName, slog.LevelInfo, 5*time.Second, slog.Default())
	
	logHandler = victoria.NewMultiHandler(logHandler, vlogHandler)
	logger := slog.New(logHandler)

	logger.Info("Запуск Dockrunner агента",
		"Версия", config.Version,
		"Агент", cfg.AgentName,
		"rabbitmq", config.RedactURL(cfg.RabbitURL),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Подключение к БД
	db, err := store.Open(cfg.SQLitePath)
	if err != nil {
		logger.Error("Ошибка открытия SQLite", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	//Подключение к Docker
	dockCli, err := docker.New()
	if err != nil {
		logger.Error("Docker не найден", "error", err)
		os.Exit(1)
	}
	defer func() { _ = dockCli.Close() }()

	if _, err := dockCli.RunningIDs(ctx); err != nil {
		logger.Error("Ошибка проверки соединения с Docker", "error", err)
		os.Exit(1)
	}

	//Подключение к RabbitMQ
	queueClient := connectQueue(ctx, cfg, logger)
	if queueClient != nil {
		defer func() { _ = queueClient.Close() }()
	}

	streamer := logs.New(dockCli, logger)
	builds := buildlog.New(db)
	reattach(ctx, logger, db, dockCli, streamer)

	if queueClient != nil {
		handler := worker.NewHandler(cfg.AgentName, cfg.Workspace, db, dockCli, queueClient, logger, builds, streamer.Attach, streamer.Detach)
		handler.SetHostWorkspace(cfg.HostWorkspace)
		if err := queueClient.Consume(queue.AgentQueue(cfg.AgentName), 4, handler.Handle); err != nil {
			logger.Error("consume tasks failed", "error", err)
			os.Exit(1)
		}
	}

	//Подключение к Victoria Metrics
	var metricsWriter *victoria.MetricsWriter
	metricsWriter = victoria.NewMetricsWriter(cfg.VictoriaMetricsURL, cfg.VictoriaToken, 5*time.Second, logger)


	//Получение данных о сервере
	serverGeo := geo.Detect(ctx, logger)

	//Запуск горутины Heartbeat
	go heartbeat.Start(ctx, logger, cfg, db, dockCli, metricsWriter, &serverGeo)

	//Настройка http сервера
	apiSrv := api.New(cfg, db, dockCli, streamer, builds, logger.Info)
	httpSrv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      apiSrv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	//Запуск http сервера
	go func() {
		logger.Info("API агента запущено", "Адрес", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server failed", "error", err)
		}
	}()

	//Ожидание сигнала на выключение
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("Получен сигнал на выключение", "signal", sig.String())

	//Выключение агента
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Ошибка выключения API", "error", err)
	}
	streamer.Stop()
	vlogHandler.Close()
	
	logger.Info("Выключение агента")
}

// connectQueue подключает RabbitMQ. Без очереди возвращает nil —
// задачи и стрим логов недоступны, HTTP API продолжает работать.
func connectQueue(ctx context.Context, cfg *config.Config, logger *slog.Logger) *queue.Client {
	queueClient, err := queue.Dial(ctx, cfg.RabbitURL, cfg.RabbitTLSSkipVerify, logger)
	if err != nil {
		logger.Warn("rabbitmq not available, tasks and log streaming disabled", "error", err)
		return nil
	}
	if err := queueClient.DeclareTopology(cfg.AgentName); err != nil {
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
