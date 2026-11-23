package heartbeat

import (
	"context"
	"database/sql"
	"time"

	"log/slog"

	"dockrunner_agent/internal/config"
	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/geo"
	"dockrunner_agent/internal/victoria"
)

type UserStorage struct {
	Owner        string  `json:"owner"`
	ContainersGB float64 `json:"containers_gb"`
	FilesGB      float64 `json:"files_gb"`
}

type Report struct {
	// AgentID оставлен для совместимости со старыми централями: агент
	// идентифицируется именем (AGENT_NAME), оно же уходит в agent_name.
	AgentID     string        `json:"agent_id"`
	AgentName   string        `json:"agent_name"`
	Hostname    string        `json:"hostname"`
	IP          string        `json:"ip"`
	Description string        `json:"description"`
	City        string        `json:"city"`
	Lat         float64       `json:"lat"`
	Lng         float64       `json:"lng"`
	ApiURL      string        `json:"api_url"`
	CPUPercent  float64       `json:"cpu_percent"`
	CPUTotal    int           `json:"cpu_total"`
	CPUMhz      int           `json:"cpu_mhz"`
	MemUsedGB   float64       `json:"mem_used_gb"`
	MemTotalGB  float64       `json:"mem_total_gb"`
	MemSpeedMhz int           `json:"mem_speed_mhz"`
	DiskUsedGB  float64       `json:"disk_used_gb"`
	DiskTotalGB float64       `json:"disk_total_gb"`
	Containers  int           `json:"containers"`
	Storage     []UserStorage `json:"storage"`
	NetDownMbps float64       `json:"net_down_mbps"`
	NetUpMbps   float64       `json:"net_up_mbps"`
}

func Start(ctx context.Context, logger *slog.Logger, cfg *config.Config, db *sql.DB, dock *docker.Client, metrics *victoria.MetricsWriter, geo *geo.Location) {
	// Send the first heartbeat immediately so the central server receives the
	// IP/location detected during agent startup without waiting one full interval.
	send := func() {
		report, err := Collect(ctx, db, dock, cfg, geo)
		if err != nil {
			logger.Warn("heartbeat metrics collection failed", "error", err)
		}
		if metrics != nil {
			line := victoria.NodeMetricsLine(report.AgentID, cfg.AgentName,
				report.CPUPercent, report.MemUsedGB, report.MemTotalGB,
				report.DiskUsedGB, report.DiskTotalGB, report.Containers,
				report.NetDownMbps, report.NetUpMbps)
			if err := metrics.Write(ctx, line); err != nil {
				logger.Warn("victoria metrics write failed", "error", err)
			}
		}
		if err := sendOnce(ctx, cfg.CentralURL, cfg.Token, report); err != nil {
			logger.Warn("heartbeat failed", "error", err)
		}
	}

	send()
	// HEARTBEAT_INTERVAL может быть пустым или заданным числом секунд без
	// суффикса ("15") — такой интервал не парсится, и NewTicker(0) паникует.
	interval := cfg.Heartbeat
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}
