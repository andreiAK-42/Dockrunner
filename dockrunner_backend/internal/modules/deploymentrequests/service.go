package deploymentrequests

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/balances"
	"dockrunner_backend/internal/modules/containers"
	"dockrunner_backend/internal/modules/pricing"
)

var (
	ErrNotFound     = errors.New("request not found")
	ErrInvalidState = errors.New("invalid request state")
)

type Service struct {
	repo        *Repository
	pricingRepo *pricing.Repository
	containers  *containers.Service
	balances    *balances.Service
	logger      *zap.Logger
}

func NewService(repo *Repository, pricingRepo *pricing.Repository, containers *containers.Service, balances *balances.Service, logger *zap.Logger) *Service {
	return &Service{repo: repo, pricingRepo: pricingRepo, containers: containers, balances: balances, logger: logger}
}

// ServiceCount — число контейнеров заявки (1 для Dockerfile/образа).
func ServiceCount(req *Request) int {
	if len(req.Services) == 0 {
		return 1
	}
	return len(req.Services)
}

func (s *Service) Create(ctx context.Context, req *Request) error {
	if req.CPUCores <= 0 || req.MemoryGB <= 0 || req.DiskGB <= 0 || req.DurationHours <= 0 {
		return errors.New("resources and duration must be positive")
	}
	if req.RepoURL != "" && req.Branch == "" {
		return errors.New("branch is required when repo_url is set")
	}
	return s.repo.Create(ctx, req)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Request, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) ListByOwner(ctx context.Context, owner string) ([]Request, error) {
	return s.repo.ListByOwner(ctx, owner)
}

func (s *Service) ListPending(ctx context.Context) ([]Request, error) {
	return s.repo.ListPending(ctx)
}

func (s *Service) ListAll(ctx context.Context) ([]Request, error) {
	return s.repo.ListAll(ctx)
}

// CalculatePrice считает стоимость заявки: цена одного контейнера × число
// сервисов (лимиты из заявки действуют на КАЖДЫЙ контейнер).
func (s *Service) CalculatePrice(ctx context.Context, req *Request) (float64, error) {
	rates, err := s.pricingRepo.GetFor(ctx, req.NodeID)
	if err != nil {
		return 0, err
	}
	base := pricing.CalcPrice(rates, req.CPUCores, req.MemoryGB, req.DiskGB, req.DurationHours)
	return math.Round(base*float64(ServiceCount(req))*100) / 100, nil
}

// Approve одобряет заявку: сначала списывает цену с баланса, потом запускает контейнеры.
// Если списание не удалось — контейнер не запускается, заявка остаётся pending.
// Если запуск не удался после списания — деньги возвращаются.
func (s *Service) Approve(ctx context.Context, id uuid.UUID, adminLogin string) (string, error) {
	req, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", ErrNotFound
	}
	if req.Status != "pending" {
		return "", ErrInvalidState
	}
	if req.Price <= 0 {
		return "", errors.New("price must be positive — set price before approval")
	}

	// For update requests, we don't charge again (already paid for original deployment)
	if req.Type == "update" {
		return s.approveUpdate(ctx, req, adminLogin)
	}

	// For create requests, charge and run new deployment
	if _, err := s.balances.DeductFunds(ctx, req.OwnerLogin, req.Price, "деплой "+req.AppName); err != nil {
		return "", err
	}
	ports := make([]containers.PortSpec, 0, len(req.Ports))
	for _, p := range req.Ports {
		ports = append(ports, containers.PortSpec{HostPort: p.HostPort, ContainerPort: p.ContainerPort})
	}
	volumes := make([]containers.VolumeSpec, 0, len(req.Volumes))
	for _, v := range req.Volumes {
		volumes = append(volumes, containers.VolumeSpec{Name: v.Name, HostPath: v.HostPath, ContainerPath: v.ContainerPath})
	}
	deploymentID, err := s.containers.Run(ctx, req.OwnerLogin, containers.RunInput{
		AppName:       req.AppName,
		RepoURL:       req.RepoURL,
		Image:         req.Image,
		ContainerName: req.ContainerName,
		NodeID:        req.NodeID,
		Ports:         ports,
		Volumes:       volumes,
		Env:           req.Env,
		CPUCores:      req.CPUCores,
		MemoryGB:      req.MemoryGB,
		DiskGB:        req.DiskGB,
		DurationHours: req.DurationHours,
		Services:      req.Services,
		BuildMode:     req.BuildMode,
		Branch:        req.Branch,
	})
	if err != nil {
		_, _ = s.balances.AddFunds(ctx, req.OwnerLogin, req.Price, "возврат за неудачный деплой "+req.AppName)
		return "", err
	}
	if err := s.repo.UpdateStatus(ctx, id, "approved", "", adminLogin, deploymentID); err != nil {
		return "", err
	}
	return deploymentID, nil
}

// approveUpdate handles approval of an update request - recreates existing container with new params
func (s *Service) approveUpdate(ctx context.Context, req *Request, adminLogin string) (string, error) {
	// No charge for updates - recreate the existing container with new parameters
	ports := make([]containers.PortSpec, 0, len(req.Ports))
	for _, p := range req.Ports {
		ports = append(ports, containers.PortSpec{HostPort: p.HostPort, ContainerPort: p.ContainerPort})
	}
	volumes := make([]containers.VolumeSpec, 0, len(req.Volumes))
	for _, v := range req.Volumes {
		volumes = append(volumes, containers.VolumeSpec{Name: v.Name, HostPath: v.HostPath, ContainerPath: v.ContainerPath})
	}

	// Use the restart_policy from the request (default to "unless-stopped" if not set)
	restartPolicy := req.RestartPolicy
	if restartPolicy == "" {
		restartPolicy = "unless-stopped"
	}

	// Recreate the container with new parameters
	responseBody, err := s.containers.RecreateContainer(ctx, req.OwnerLogin, req.ExistingDeploymentID, map[string]any{
		"image":           req.Image,
		"cpu_cores":       req.CPUCores,
		"memory_gb":       req.MemoryGB,
		"disk_gb":         req.DiskGB,
		"ports":           ports,
		"env":             req.Env,
		"volumes":         volumes,
		"restart_policy":  restartPolicy,
		"duration_hours":  req.DurationHours,
	})
	if err != nil {
		return "", err
	}

	// The responseBody contains the updated container info
	var updated struct {
		Image string `json:"image"`
	}
	if err := json.Unmarshal(responseBody, &updated); err == nil && updated.Image != "" {
		req.Image = updated.Image
	}

	if err := s.repo.UpdateStatus(ctx, req.ID, "approved", "", adminLogin, req.ExistingDeploymentID); err != nil {
		return "", err
	}
	return req.ExistingDeploymentID, nil
}

func (s *Service) Reject(ctx context.Context, id uuid.UUID, adminLogin, comment string) error {
	req, err := s.repo.Get(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if req.Status != "pending" {
		return ErrInvalidState
	}
	return s.repo.UpdateStatus(ctx, id, "rejected", comment, adminLogin, "")
}

func (s *Service) SetPrice(ctx context.Context, id uuid.UUID, price float64) error {
	if price < 0 {
		return errors.New("price must not be negative")
	}
	return s.repo.UpdatePrice(ctx, id, price)
}

func CalcPriceStatic(rates pricing.Rates, cpuCores, memoryGB, diskGB, hours float64) float64 {
	hourly := cpuCores*rates.CpuPerCoreHour + memoryGB*rates.MemPerGBHour + diskGB*rates.DiskPerGBHour
	return math.Round(hourly*hours*100) / 100
}
