package quotas

import (
	"context"

	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/admin"
)

type Service struct {
	repo     *Repository
	logger   *zap.Logger
	portsMin int
	portsMax int
}

func NewService(repo *Repository, logger *zap.Logger, portsMin, portsMax int) *Service {
	if portsMin < 1 {
		portsMin = 1
	}
	if portsMax > 65535 {
		portsMax = 65535
	}
	if portsMin > portsMax {
		portsMax = portsMin
	}
	return &Service{repo: repo, logger: logger, portsMin: portsMin, portsMax: portsMax}
}

func (s *Service) Get(ctx context.Context, owner string) (*Quotas, error) {
	plan := admin.FindPlan(s.repo.userPlan(ctx, owner))
	return &Quotas{
		AppsUsed:   s.repo.appsUsed(ctx, owner),
		AppsLimit:  plan.Apps,
		PortsUsed:  s.repo.portsUsed(ctx, owner),
		PortsLimit: plan.Ports,
		PortsMin:   s.portsMin,
		PortsMax:   s.portsMax,
		EnvUsed:    s.repo.envUsed(ctx, owner),
		EnvLimit:   plan.EnvConfigs,
		StorageGB:  s.repo.storageUsed(ctx, owner),
		StorageLim: plan.DbStorageGB,
	}, nil
}
