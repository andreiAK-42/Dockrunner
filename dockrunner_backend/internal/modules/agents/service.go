package agents

import (
	"context"
	"time"

	"go.uber.org/zap"

	"dockrunner_backend/pkg/database/influxdb"
)

type Service struct {
	repo   *Repository
	influx *influxdb.Client
	logger *zap.Logger
}

func NewService(repo *Repository, influx *influxdb.Client, logger *zap.Logger) *Service {
	return &Service{repo: repo, influx: influx, logger: logger}
}

type HeartbeatInput struct {
	AgentID       string       `json:"agent_id"`
	Hostname      string       `json:"hostname"`
	IP            string       `json:"ip"`
	City          string       `json:"city"`
	Lat           float64      `json:"lat"`
	Lng           float64      `json:"lng"`
	ApiURL        string       `json:"api_url"`
	CPUTotal      int            `json:"cpu_total"`
	CPUPercent    float64        `json:"cpu_percent"`
	CPUMhz        int            `json:"cpu_mhz"`
	MemUsedGB     float64      `json:"mem_used_gb"`
	MemTotalGB    float64      `json:"mem_total_gb"`
	MemType       string       `json:"mem_type"`
	MemSpeedMhz   int            `json:"mem_speed_mhz"`
	DiskUsedGB    float64      `json:"disk_used_gb"`
	DiskTotalGB   float64      `json:"disk_total_gb"`
	DiskType      string       `json:"disk_type"`
	Containers    int          `json:"containers"`
	Description   string       `json:"description"`
	Storage       []StorageRow `json:"storage"`
	NetDownMbps   float64      `json:"net_down_mbps"`
	NetUpMbps     float64      `json:"net_up_mbps"`
}

// resolveAgentName возвращает имя агента (теперь это просто AgentID).
func resolveAgentName(in HeartbeatInput) string {
	return in.AgentID
}

func (s *Service) Heartbeat(ctx context.Context, in HeartbeatInput) error {
	name := resolveAgentName(in)
	if err := s.repo.UpsertHeartbeat(ctx, Agent{
		ID:            in.AgentID,
		Name:          name,
		IP:            in.IP,
		City:          in.City,
		Lat:           in.Lat,
		Lng:           in.Lng,
		ApiURL:        in.ApiURL,
		CPUTotal:      in.CPUTotal,
		CPUPercent:    in.CPUPercent,
		CPUMhz:        in.CPUMhz,
		MemUsedGB:     in.MemUsedGB,
		MemTotalGB:    in.MemTotalGB,
		MemType:       in.MemType,
		MemSpeedMhz:   in.MemSpeedMhz,
		DiskUsedGB:    in.DiskUsedGB,
		DiskTotalGB:   in.DiskTotalGB,
		DiskType:      in.DiskType,
		Containers:    in.Containers,
		Description:   in.Description,
		NetDownMbps:   in.NetDownMbps,
		NetUpMbps:     in.NetUpMbps,
		LastSeen:      time.Now().UTC(),
	}); err != nil {
		return err
	}
	if s.influx != nil {
		_ = s.influx.WriteServerMetrics(ctx, influxdb.ServerMetrics{
			ServerID: in.AgentID,
			CPU:      in.CPUPercent,
			MemUsed:  in.MemUsedGB,
			MemTotal: in.MemTotalGB,
		})
	}
	return s.repo.ReplaceStorage(ctx, in.AgentID, in.Storage)
}

func (s *Service) List(ctx context.Context) ([]Agent, error) {
	agents, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if agents == nil {
		agents = []Agent{}
	}
	return agents, nil
}
