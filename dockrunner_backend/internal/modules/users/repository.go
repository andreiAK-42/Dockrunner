package users

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type User struct {
	ID          uuid.UUID  `json:"id"`
	Login       string     `json:"login"`
	Role        string     `json:"role"`
	Blocked     bool       `json:"blocked"`
	IsSuperuser bool       `json:"is_superuser"`
	PlanID      string     `json:"plan_id"`
	PlanExpires *time.Time `json:"plan_expires"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

const userColumns = `id, login, role, blocked, is_superuser, plan_id, plan_expires, created_at, updated_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Login, &u.Role, &u.Blocked, &u.IsSuperuser, &u.PlanID, &u.PlanExpires, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]User, int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Login, &u.Role, &u.Blocked, &u.IsSuperuser, &u.PlanID, &u.PlanExpires, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&total)
	return total, err
}

func (r *Repository) Create(ctx context.Context, login, hash, role, planID string, superuser bool) (*User, error) {
	id := uuid.New()
	row := r.pool.QueryRow(ctx, `INSERT INTO users (id, login, password_hash, role, plan_id, is_superuser)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+userColumns, id, login, hash, role, planID, superuser)
	return scanUser(row)
}

func (r *Repository) GetByLogin(ctx context.Context, login string) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE login = $1`, login)
	return scanUser(row)
}

func (r *Repository) EnsureSuperuser(ctx context.Context, login string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET is_superuser = TRUE, updated_at = now() WHERE login = $1`, login)
	return err
}

func (r *Repository) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`, hash, id)
	return err
}

func (r *Repository) UpdateRole(ctx context.Context, id uuid.UUID, role string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET role = $1, updated_at = now() WHERE id = $2`, role, id)
	return err
}

func (r *Repository) SetBlocked(ctx context.Context, id uuid.UUID, blocked bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET blocked = $1, updated_at = now() WHERE id = $2`, blocked, id)
	return err
}

func (r *Repository) SetPlan(ctx context.Context, id uuid.UUID, planID string, expires *time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET plan_id = $1, plan_expires = $2, updated_at = now() WHERE id = $3`, planID, expires, id)
	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}
