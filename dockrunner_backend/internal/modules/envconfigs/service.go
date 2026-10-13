package envconfigs

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

var ErrLimit = errors.New("env configs limit reached")

type Service struct {
	repo   *Repository
	logger *zap.Logger
}

func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) List(ctx context.Context, owner string) ([]EnvConfig, error) {
	return s.repo.ListByOwner(ctx, owner)
}

func (s *Service) Create(ctx context.Context, owner, name, description string, vars []EnvVar) (*EnvConfig, error) {
	total, err := s.repo.CountByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}
	if total >= maxPerUser {
		return nil, ErrLimit
	}
	return s.repo.Create(ctx, owner, name, description, vars)
}

func (s *Service) Update(ctx context.Context, owner, id, name, description string, vars []EnvVar) (*EnvConfig, error) {
	return s.repo.Update(ctx, owner, id, name, description, vars)
}

func (s *Service) Delete(ctx context.Context, owner, id string) error {
	return s.repo.Delete(ctx, owner, id)
}
