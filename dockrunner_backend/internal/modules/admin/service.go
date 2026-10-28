package admin

import (
	"context"
	"encoding/json"
	"errors"

	"go.uber.org/zap"

	"dockrunner_backend/internal/oplock"
	"dockrunner_backend/pkg/queue"
)

type Plan struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Price       string  `json:"price"`
	Apps        int     `json:"apps"`
	Domains     int     `json:"domains"`
	Ports       int     `json:"ports"`
	EnvConfigs  int     `json:"env_configs"`
	DbStorageGB float64 `json:"db_storage_gb"`
}

var plans = []Plan{
	{ID: "student", Name: "Студенческий", Price: "Бесплатно", Apps: 10, Domains: 5, Ports: 5, EnvConfigs: 10, DbStorageGB: 10},
	{ID: "basic", Name: "Базовый", Price: "990 ₽/мес", Apps: 30, Domains: 15, Ports: 10, EnvConfigs: 30, DbStorageGB: 50},
	{ID: "pro", Name: "Про", Price: "2990 ₽/мес", Apps: 100, Domains: 50, Ports: 25, EnvConfigs: 100, DbStorageGB: 200},
}

type Service struct {
	repo   *Repository
	queue  *queue.Client
	logger *zap.Logger
	ops    *oplock.Tracker
}

func NewService(repo *Repository, queue *queue.Client, logger *zap.Logger, ops *oplock.Tracker) *Service {
	if ops == nil {
		ops = &oplock.Tracker{}
	}
	return &Service{repo: repo, queue: queue, logger: logger, ops: ops}
}

func (s *Service) Plans() []Plan {
	return plans
}

func FindPlan(id string) Plan {
	for _, p := range plans {
		if p.ID == id {
			return p
		}
	}
	return plans[0]
}

func (s *Service) ListContainers(ctx context.Context) ([]ContainerRecord, error) {
	return s.repo.ListContainers(ctx)
}

func (s *Service) ContainerAction(ctx context.Context, id, action string) error {
	if s.queue == nil {
		return errors.New("queue unavailable")
	}
	rec, err := s.repo.GetContainer(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ops.Acquire(rec.OwnerLogin, rec.AppName, action); err != nil {
		return err
	}
	taskType := action
	body, _ := json.Marshal(map[string]string{
		"type":          taskType,
		"deployment_id": rec.ID,
		"app_id":        rec.AppID,
		"app_name":      rec.AppName,
		"owner":         rec.OwnerLogin,
	})
	if err := s.queue.Publish(ctx, "agent."+rec.NodeID, body); err != nil {
		s.ops.Release(rec.OwnerLogin, rec.AppName)
		return err
	}
	return nil
}

func (s *Service) RebootServer(ctx context.Context, id string) error {
	if s.queue == nil {
		return errors.New("queue unavailable")
	}
	body, _ := json.Marshal(map[string]string{"type": "reboot"})
	return s.queue.Publish(ctx, "agent."+id, body)
}

// UpdateContainerParams updates container parameters and triggers a recreate
// with the new parameters. This is an admin-only operation.
func (s *Service) UpdateContainerParams(ctx context.Context, id string, params map[string]any) error {
	if s.queue == nil {
		return errors.New("queue unavailable")
	}

	rec, err := s.repo.GetContainer(ctx, id)
	if err != nil {
		return err
	}

	// Acquire operation lock
	if err := s.ops.Acquire(rec.OwnerLogin, rec.AppName, "admin_update"); err != nil {
		return err
	}
	defer s.ops.Release(rec.OwnerLogin, rec.AppName)

	// Update the database with new parameters
	if err := s.repo.UpdateContainerParams(ctx, id, params); err != nil {
		return err
	}

	// Send recreate task to agent with new parameters
	// We need to fetch the current full spec and merge with new params
	body, _ := json.Marshal(map[string]any{
		"type":           "recreate",
		"deployment_id":  rec.ID,
		"app_name":       rec.AppName,
		"owner":          rec.OwnerLogin,
		"image":          params["image"],
		"cpu_cores":      params["cpu_cores"],
		"memory_gb":      params["memory_gb"],
		"disk_gb":        params["disk_gb"],
		"ports":          params["ports"],
		"env":            params["env"],
		"volumes":        params["volumes"],
		"restart_policy": params["restart_policy"],
	})

	if err := s.queue.Publish(ctx, "agent."+rec.NodeID, body); err != nil {
		return err
	}

	return nil
}

type AgentEvent struct {
	Type               string        `json:"type"`
	AgentID            string        `json:"agent_id"`
	AppID              string        `json:"app_id"`
	AppName            string        `json:"app_name"`
	OwnerLogin         string        `json:"owner_login"`
	Image              string        `json:"image"`
	DeploymentID       string        `json:"deployment_id"`
	DetectedDockerfile bool          `json:"detected_dockerfile"`
	ComposeFile        string        `json:"compose_file"`
	Services           []string      `json:"services"`
	ParentApp          string        `json:"parent_app"`
	Ports              []PortMapping `json:"ports"`
	Message            string        `json:"msg"`
}

// OpApp возвращает приложение для oplock: у сервис-контейнеров это родитель.
func (e AgentEvent) OpApp() string {
	if e.ParentApp != "" {
		return e.ParentApp
	}
	return e.AppName
}

// mergeResources подтягивает лимиты/expires и параметры запуска из ранее созданной записи Run,
// чтобы deploy.completed не затирал их нулями.
func (s *Service) mergeResources(ctx context.Context, c ContainerRecord) ContainerRecord {
	if existing, err := s.repo.GetContainer(ctx, c.ID); err == nil && existing != nil {
		c.CPUCores = existing.CPUCores
		c.MemoryGB = existing.MemoryGB
		c.DiskGB = existing.DiskGB
		c.ExpiresAt = existing.ExpiresAt
		// Preserve launch parameters that are not returned by the agent
		c.Volumes = existing.Volumes
		c.Env = existing.Env
		c.RestartPolicy = existing.RestartPolicy
	}
	return c
}

func (s *Service) ApplyEvent(ctx context.Context, e AgentEvent) error {
	switch e.Type {
	case "deploy.completed":
		return s.repo.UpsertContainer(ctx, s.mergeResources(ctx, ContainerRecord{
			ID: e.DeploymentID, AppID: e.AppID, AppName: e.AppName, OwnerLogin: e.OwnerLogin,
			Image: e.Image, Status: "running", NodeID: e.AgentID, Ports: e.Ports,
		}))
	case "deploy.failed":
		return s.repo.UpsertContainer(ctx, s.mergeResources(ctx, ContainerRecord{
			ID: e.DeploymentID, AppID: e.AppID, AppName: e.AppName, OwnerLogin: e.OwnerLogin,
			Image: e.Image, Status: "error", NodeID: e.AgentID, Ports: e.Ports,
		}))
	case "container.stopped":
		return s.repo.SetContainerStatus(ctx, e.DeploymentID, "stopped")
	case "container.restarted":
		return s.repo.SetContainerStatus(ctx, e.DeploymentID, "running")
	case "container.removed":
		return s.repo.DeleteContainer(ctx, e.DeploymentID)
	case "op.failed":
		return s.repo.SetContainerStatus(ctx, e.DeploymentID, "error")
	default:
		return nil
	}
}
