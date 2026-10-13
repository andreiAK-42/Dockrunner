package balances

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrUserNotFound      = errors.New("user not found")
)

type Service struct {
	repo   *Repository
	logger *zap.Logger
}

func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) GetBalance(ctx context.Context, owner string) (float64, error) {
	return s.repo.GetBalance(ctx, owner)
}

func (s *Service) AddFunds(ctx context.Context, owner string, amount float64, reason string) (float64, error) {
	if amount <= 0 {
		return 0, errors.New("amount must be positive")
	}
	return s.repo.AddBalance(ctx, owner, amount, reason)
}

func (s *Service) DeductFunds(ctx context.Context, owner string, amount float64, reason string) (float64, error) {
	if amount <= 0 {
		return 0, errors.New("amount must be positive")
	}
	balance, err := s.repo.GetBalance(ctx, owner)
	if err != nil {
		return 0, ErrUserNotFound
	}
	if balance < amount {
		return 0, ErrInsufficientFunds
	}
	return s.repo.DeductBalance(ctx, owner, amount, reason)
}

func (s *Service) ListTransactions(ctx context.Context, owner string, page, limit int) ([]Transaction, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	return s.repo.ListTransactions(ctx, owner, limit, offset)
}

func (s *Service) ListAllTransactions(ctx context.Context, page, limit int) ([]Transaction, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	return s.repo.ListAllTransactions(ctx, limit, offset)
}

func (s *Service) SetBalance(ctx context.Context, owner string, balance float64) error {
	if balance < 0 {
		return errors.New("balance must not be negative")
	}
	return s.repo.SetBalance(ctx, owner, balance)
}