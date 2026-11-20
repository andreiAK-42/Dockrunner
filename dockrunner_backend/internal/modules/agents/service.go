package agents

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ErrNoAgentID — в heartbeat нет ни agent_id, ни agent_name.
var ErrNoAgentID = errors.New("agent id is empty")

type Service struct {
	repo   *Repository
	logger *zap.Logger
}

func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

type HeartbeatInput struct {
	// AgentID — старый идентификатор узла. Новые агенты присылают только
	// AgentName (AGENT_NAME), поэтому id берётся из него, если agent_id пуст.
	AgentID     string       `json:"agent_id"`
	AgentName   string       `json:"agent_name"`
	Hostname    string       `json:"hostname"`
	IP          string       `json:"ip"`
	City        string       `json:"city"`
	Lat         float64      `json:"lat"`
	Lng         float64      `json:"lng"`
	ApiURL      string       `json:"api_url"`
	CPUTotal    int          `json:"cpu_total"`
	CPUPercent  float64      `json:"cpu_percent"`
	CPUMhz      int          `json:"cpu_mhz"`
	MemUsedGB   float64      `json:"mem_used_gb"`
	MemTotalGB  float64      `json:"mem_total_gb"`
	MemType     string       `json:"mem_type"`
	MemSpeedMhz int          `json:"mem_speed_mhz"`
	DiskUsedGB  float64      `json:"disk_used_gb"`
	DiskTotalGB float64      `json:"disk_total_gb"`
	DiskType    string       `json:"disk_type"`
	Containers  int          `json:"containers"`
	Description string       `json:"description"`
	Storage     []StorageRow `json:"storage"`
	NetDownMbps float64      `json:"net_down_mbps"`
	NetUpMbps   float64      `json:"net_up_mbps"`
}

// resolveAgentID возвращает идентификатор узла: agent_id, а если агент его
// больше не присылает (остался только AGENT_NAME) — agent_name.
func resolveAgentID(in HeartbeatInput) string {
	if id := strings.TrimSpace(in.AgentID); id != "" {
		return id
	}
	return strings.TrimSpace(in.AgentName)
}

// resolveAgentName возвращает отображаемое имя узла (по умолчанию — id).
func resolveAgentName(in HeartbeatInput) string {
	if name := strings.TrimSpace(in.AgentName); name != "" {
		return name
	}
	return resolveAgentID(in)
}

func (s *Service) Heartbeat(ctx context.Context, in HeartbeatInput) error {
	id := resolveAgentID(in)
	name := resolveAgentName(in)
	if id == "" {
		return ErrNoAgentID
	}
	// Агент мог сменить идентификатор (например, оставив только AGENT_NAME):
	// переносим запись узла вместе со ссылками, иначе приложения «повиснут»
	// на старом node_id и центр будет считать узел офлайн.
	if err := s.repo.EnsureAgentIdentity(ctx, id, name, in.ApiURL); err != nil {
		s.logger.Warn("agent identity migration failed", zap.String("agent", id), zap.Error(err))
	}
	if err := s.repo.UpsertHeartbeat(ctx, Agent{
		ID:          id,
		Name:        name,
		IP:          in.IP,
		City:        in.City,
		Lat:         in.Lat,
		Lng:         in.Lng,
		ApiURL:      in.ApiURL,
		CPUTotal:    in.CPUTotal,
		CPUPercent:  in.CPUPercent,
		CPUMhz:      in.CPUMhz,
		MemUsedGB:   in.MemUsedGB,
		MemTotalGB:  in.MemTotalGB,
		MemType:     in.MemType,
		MemSpeedMhz: in.MemSpeedMhz,
		DiskUsedGB:  in.DiskUsedGB,
		DiskTotalGB: in.DiskTotalGB,
		DiskType:    in.DiskType,
		Containers:  in.Containers,
		Description: in.Description,
		NetDownMbps: in.NetDownMbps,
		NetUpMbps:   in.NetUpMbps,
		LastSeen:    time.Now().UTC(),
	}); err != nil {
		return err
	}
	// Метрики узла агент пишет сам напрямую в VictoriaMetrics —
	// здесь heartbeat только обновляет Postgres.
	return s.repo.ReplaceStorage(ctx, id, in.Storage)
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
