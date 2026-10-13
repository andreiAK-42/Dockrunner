package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dockrunner_backend/pkg/security"
)

// ErrForbidden — обычный админ трогает админа: нужен главный (суперпользователь).
var ErrForbidden = errors.New("только главный администратор может управлять администраторами")

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

// checkManage проверяет, может ли requester управлять target:
// админами (и суперадмином) управляет только главный.
func checkManage(requester, target *User) error {
	if target.IsSuperuser && requester.ID != target.ID {
		return ErrForbidden
	}
	if target.Role == "admin" && !requester.IsSuperuser {
		return ErrForbidden
	}
	return nil
}

func (s *Service) requester(ctx context.Context, login string) (*User, error) {
	if login == "" {
		return nil, ErrForbidden
	}
	return s.repo.GetByLogin(ctx, login)
}

func (s *Service) UpdateRole(ctx context.Context, actorLogin, id, role string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	actor, err := s.requester(ctx, actorLogin)
	if err != nil {
		return err
	}
	target, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return err
	}
	if err := checkManage(actor, target); err != nil {
		return err
	}
	if role == "admin" && !actor.IsSuperuser {
		return ErrForbidden
	}
	return s.repo.UpdateRole(ctx, uid, role)
}

func (s *Service) SetBlocked(ctx context.Context, actorLogin, id string, blocked bool) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	actor, err := s.requester(ctx, actorLogin)
	if err != nil {
		return err
	}
	target, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return err
	}
	// Главного не блокирует никто, даже он сам.
	if target.IsSuperuser {
		return ErrForbidden
	}
	if err := checkManage(actor, target); err != nil {
		return err
	}
	return s.repo.SetBlocked(ctx, uid, blocked)
}

func (s *Service) SetPassword(ctx context.Context, actorLogin, id, password string, bcryptCost int) error {
	if len(password) < 6 || len(password) > 72 {
		return errors.New("password must be 6..72 chars")
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	actor, err := s.requester(ctx, actorLogin)
	if err != nil {
		return err
	}
	target, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return err
	}
	if err := checkManage(actor, target); err != nil {
		return err
	}
	hash, err := security.HashPassword(password, bcryptCost)
	if err != nil {
		return err
	}
	return s.repo.SetPassword(ctx, uid, hash)
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
	return s.repo.Create(ctx, login, hash, role, planID, false)
}

func (s *Service) Delete(ctx context.Context, actorLogin, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	actor, err := s.requester(ctx, actorLogin)
	if err != nil {
		return err
	}
	target, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return err
	}
	if target.IsSuperuser {
		return ErrForbidden
	}
	if err := checkManage(actor, target); err != nil {
		return err
	}
	return s.repo.Delete(ctx, uid)
}
