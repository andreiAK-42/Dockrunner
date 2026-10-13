package users

import (
	"context"

	"dockrunner_backend/pkg/security"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func SeedAdmin(ctx context.Context, pool *pgxpool.Pool, login, password string, bcryptCost int, logger *zap.Logger) {
	if pool == nil || login == "" || password == "" {
		return
	}
	repo := NewRepository(pool, logger)
	total, err := repo.Count(ctx)
	if err != nil {
		logger.Warn("seed admin: count failed", zap.Error(err))
		return
	}
	if total > 0 {
		return
	}
	hash, err := security.HashPassword(password, bcryptCost)
	if err != nil {
		logger.Warn("seed admin: hash failed", zap.Error(err))
		return
	}
	if _, err := repo.Create(ctx, login, hash, "admin", "pro", true); err != nil {
		logger.Warn("seed admin: create failed", zap.Error(err))
		return
	}
	logger.Info("seed admin created", zap.String("login", login))
}

// EnsureSuperuser помечает логин из .env главным администратором.
// Нужно для баз, созданных до появления флага is_superuser.
func EnsureSuperuser(ctx context.Context, pool *pgxpool.Pool, login string, logger *zap.Logger) {
	if pool == nil || login == "" {
		return
	}
	repo := NewRepository(pool, logger)
	if err := repo.EnsureSuperuser(ctx, login); err != nil {
		logger.Warn("ensure superuser failed", zap.Error(err))
	}
}
