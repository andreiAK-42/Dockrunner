package users

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dockrunner_backend/pkg/security"
)

type Service struct {
	repo   *Repository
	logger *zap.Logger
}

func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) List(ctx context.Context, page, limit int) ([]User, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, uid)
}

func (s *Service) UpdateRole(ctx context.Context, id, role string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return s.repo.UpdateRole(ctx, uid, role)
}

func (s *Service) SetBlocked(ctx context.Context, id string, blocked bool) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return s.repo.SetBlocked(ctx, uid, blocked)
}

func (s *Service) SetPlan(ctx context.Context, id, planID string, expires *time.Time) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return s.repo.SetPlan(ctx, uid, planID, expires)
}

func (s *Service) AdminCreate(ctx context.Context, login, password, role, planID string, bcryptCost int) (*User, error) {
	hash, err := security.HashPassword(password, bcryptCost)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, login, hash, role, planID)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, uid)
}
