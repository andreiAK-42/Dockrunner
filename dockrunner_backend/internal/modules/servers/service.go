package servers

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	repo   *Repository
	logger *zap.Logger
}

func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) List(ctx context.Context, page, limit int) ([]Server, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) GetByID(ctx context.Context, id string) (*Server, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, uid)
}

func (s *Service) Create(ctx context.Context, name, ip, region string) (*Server, error) {
	srv := &Server{
		ID:     uuid.New(),
		Name:   name,
		IP:     ip,
		Region: region,
		Status: "pending",
	}
	if err := s.repo.Create(ctx, srv); err != nil {
		return nil, err
	}
	s.logger.Info("server created", zap.String("id", srv.ID.String()), zap.String("name", name))
	return srv, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, uid)
}
