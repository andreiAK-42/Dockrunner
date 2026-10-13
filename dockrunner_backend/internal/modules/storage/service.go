package storage

import (
	"context"

	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/agents"
)

type Usage struct {
	ContainersGB float64 `json:"containers_gb"`
	FilesGB      float64 `json:"files_gb"`
	TotalGB      float64 `json:"total_gb"`
}

type Service struct {
	agents *agents.Repository
	logger *zap.Logger
}

func NewService(agents *agents.Repository, logger *zap.Logger) *Service {
	return &Service{agents: agents, logger: logger}
}

func (s *Service) Usage(ctx context.Context, owner string) (*Usage, error) {
	containers, files, err := s.agents.UserTotals(ctx, owner)
	if err != nil {
		return nil, err
	}
	return &Usage{ContainersGB: containers, FilesGB: files, TotalGB: containers + files}, nil
}
