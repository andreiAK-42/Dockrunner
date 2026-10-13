package servers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Server struct {
	ID        uuid.UUID `json:"id" bson:"_id"`
	Name      string    `json:"name" bson:"name"`
	IP        string    `json:"ip" bson:"ip"`
	Status    string    `json:"status" bson:"status"`
	Region    string    `json:"region" bson:"region"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

// Repo uses Postgres for inventory
type Repository struct {
	pgPool *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pgPool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pgPool: pgPool, logger: logger}
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]Server, int64, error) {
	// Postgres: SELECT ... LIMIT/OFFSET + COUNT
	return []Server{}, 0, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Server, error) {
	return nil, nil
}

func (r *Repository) Create(ctx context.Context, s *Server) error {
	// In real: INSERT into servers table
	return nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return nil
}
