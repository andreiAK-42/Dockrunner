package integrations

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Account — учётка интеграции. Пароль не хранится, показывается при создании и сбросе.
type Account struct {
	ID         uuid.UUID `json:"id"`
	OwnerLogin string    `json:"ownerLogin"`
	Kind       string    `json:"kind"`
	Resource   string    `json:"resource"`
	Username   string    `json:"username"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	SizeGB     float64   `json:"sizeGB"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

func scanAccount(row pgx.Row) (*Account, error) {
	var a Account
	if err := row.Scan(&a.ID, &a.OwnerLogin, &a.Kind, &a.Resource, &a.Username, &a.Host, &a.Port, &a.SizeGB, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

const accountColumns = `id, owner_login, db_type, db_name, username, host, port, size_gb, created_at`

func (r *Repository) Create(ctx context.Context, a *Account) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO database_users
		(id, owner_login, db_type, db_name, username, host, port, size_gb, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now())`,
		a.ID, a.OwnerLogin, a.Kind, a.Resource, a.Username, a.Host, a.Port, a.SizeGB)
	return err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Account, error) {
	return scanAccount(r.pool.QueryRow(ctx, `SELECT `+accountColumns+` FROM database_users WHERE id = $1`, id))
}

func (r *Repository) GetByOwnerKind(ctx context.Context, owner, kind string) (*Account, error) {
	return scanAccount(r.pool.QueryRow(ctx, `SELECT `+accountColumns+` FROM database_users
		WHERE owner_login = $1 AND db_type = $2 ORDER BY created_at ASC LIMIT 1`, owner, kind))
}

func (r *Repository) ListByOwner(ctx context.Context, owner string) ([]Account, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+accountColumns+` FROM database_users WHERE owner_login = $1 ORDER BY created_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM database_users WHERE id = $1`, id)
	return err
}

func (r *Repository) DbUsageGB(ctx context.Context, owner string) float64 {
	var total float64
	_ = r.pool.QueryRow(ctx, `SELECT coalesce(sum(size_gb), 0) FROM database_users
		WHERE owner_login = $1 AND db_type IN ('postgres','mongo','redis')`, owner).Scan(&total)
	return total
}

func (r *Repository) StorageUsageGB(ctx context.Context, owner string) float64 {
	var containers, files float64
	_ = r.pool.QueryRow(ctx, `SELECT coalesce(sum(containers_gb), 0), coalesce(sum(files_gb), 0)
		FROM storage_usage WHERE owner_login = $1`, owner).Scan(&containers, &files)
	return containers + files + r.DbUsageGB(ctx, owner)
}

func (r *Repository) UserPlan(ctx context.Context, owner string) string {
	var plan string
	if err := r.pool.QueryRow(ctx, `SELECT plan_id FROM users WHERE login = $1`, owner).Scan(&plan); err != nil || plan == "" {
		return "student"
	}
	return plan
}
