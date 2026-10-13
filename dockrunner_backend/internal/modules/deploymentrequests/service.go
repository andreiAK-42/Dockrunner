package deploymentrequests

import (
	"context"
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

// Approve одобряет заявку: запускает контейнеры и списывает цену с баланса.
// При нехватке средств деплой best-effort удаляется, заявка остаётся pending.
func (s *Service) Approve(ctx context.Context, id uuid.UUID, adminLogin string) (string, error) {
	req, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", ErrNotFound
	}
	if req.Status != "pending" {
		return "", ErrInvalidState
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
	})
	if err != nil {
		return "", err
	}
	if _, err := s.balances.DeductFunds(ctx, req.OwnerLogin, req.Price, "деплой "+req.AppName); err != nil {
		// Деньги не списались — убираем только что созданный деплой.
		if rmErr := s.containers.RemoveDeployment(ctx, req.OwnerLogin, deploymentID); rmErr != nil {
			s.logger.Warn("approve cleanup failed", zap.String("deployment", deploymentID), zap.Error(rmErr))
		}
		return "", err
	}
	if err := s.repo.UpdateStatus(ctx, id, "approved", "", adminLogin, deploymentID); err != nil {
		return "", err
	}
	return deploymentID, nil
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
