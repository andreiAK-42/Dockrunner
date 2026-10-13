package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dockrunner_backend/pkg/security"
	"dockrunner_backend/pkg/token"
)

var (
	ErrInvalidCredentials = errors.New("invalid login or password")
	ErrUserExists         = errors.New("user already exists")
	ErrBlocked            = errors.New("user is blocked")
)

type Service struct {
	repo     *Repository
	tokenMgr *token.Manager
	refresh  *RefreshStore
	logger   *zap.Logger
	bcryptCost int
}

func NewService(repo *Repository, tokenMgr *token.Manager, refresh *RefreshStore, logger *zap.Logger, bcryptCost int) *Service {
	return &Service{
		repo:       repo,
		tokenMgr:   tokenMgr,
		refresh:    refresh,
		logger:     logger,
		bcryptCost: bcryptCost,
	}
}

type LoginResult struct {
	User         *User  `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (s *Service) Register(ctx context.Context, login, password string) (*User, error) {
	if u, _ := s.repo.FindByLogin(ctx, login); u != nil {
		return nil, ErrUserExists
	}
	hash, err := security.HashPassword(password, s.bcryptCost)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.Create(ctx, login, string(hash), "user")
	if err != nil {
		return nil, err
	}
	s.logger.Info("user registered", zap.String("login", login), zap.String("id", user.ID.String()))
	return user, nil
}

func (s *Service) Login(ctx context.Context, login, password string) (*LoginResult, error) {
	user, err := s.repo.FindByLogin(ctx, login)
	if err != nil || user == nil {
		return nil, ErrInvalidCredentials
	}
	if user.Blocked {
		return nil, ErrBlocked
	}
	if err := security.ComparePassword(user.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}
	access, err := s.tokenMgr.GenerateAccess(user.ID.String(), user.Login, user.Role)
	if err != nil {
		return nil, err
	}
	refresh, err := s.tokenMgr.GenerateRefresh(user.ID.String())
	if err != nil {
		return nil, err
	}
	if claims, err := s.tokenMgr.Parse(refresh); err == nil {
		if err := s.refresh.Store(ctx, user.ID.String(), claims.ID); err != nil {
			s.logger.Warn("refresh store failed", zap.Error(err))
		}
	}
	return &LoginResult{
		User:         user,
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    time.Now().Add(s.tokenMgr.AccessTTL()),
	}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (*User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	claims, err := s.tokenMgr.Parse(refreshToken)
	if err != nil {
		return "", "", err
	}
	user, err := s.repo.FindByID(ctx, uuid.MustParse(claims.Subject))
	if err != nil || user == nil {
		return "", "", ErrInvalidCredentials
	}
	access, err := s.tokenMgr.GenerateAccess(user.ID.String(), user.Login, user.Role)
	if err != nil {
		return "", "", err
	}
	newRefresh, err := s.tokenMgr.GenerateRefresh(user.ID.String())
	if err != nil {
		return "", "", err
	}
	newClaims, err := s.tokenMgr.Parse(newRefresh)
	if err != nil {
		return "", "", err
	}
	if err := s.refresh.CheckAndRotate(ctx, user.ID.String(), claims.ID, newClaims.ID); err != nil {
		s.logger.Warn("refresh reuse detected", zap.String("user_id", user.ID.String()))
		return "", "", err
	}
	return access, newRefresh, nil
}

func (s *Service) Logout(ctx context.Context, userID string) error {
	return s.refresh.Revoke(ctx, userID)
}
