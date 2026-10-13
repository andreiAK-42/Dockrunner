package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Login        string    `json:"login" db:"login"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"`
	Blocked      bool      `json:"blocked" db:"blocked"`
	IsSuperuser  bool      `json:"is_superuser" db:"is_superuser"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Login, &u.PasswordHash, &u.Role, &u.Blocked, &u.IsSuperuser, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) FindByLogin(ctx context.Context, login string) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, login, password_hash, role, blocked, is_superuser, created_at
		FROM users WHERE login = $1`, login)
	return scanUser(row)
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, login, password_hash, role, blocked, is_superuser, created_at
		FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *Repository) Create(ctx context.Context, login, hash, role string) (*User, error) {
	id := uuid.New()
	row := r.pool.QueryRow(ctx, `INSERT INTO users (id, login, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, login, password_hash, role, blocked, is_superuser, created_at`,
		id, login, hash, role)
	return scanUser(row)
}
