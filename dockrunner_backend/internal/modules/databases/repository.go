package databases

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DbUser — запись о пользователе СУБД. Пароль НЕ хранится:
// показывается один раз при создании.
type DbUser struct {
	ID        uuid.UUID `json:"id"`
	OwnerLogin string   `json:"ownerLogin"`
	DbType    string    `json:"dbType"`
	DbName    string    `json:"dbName"`
	Username  string    `json:"username"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	SizeGB    float64   `json:"sizeGB"`
	CreatedAt time.Time `json:"createdAt"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

func scanDbUser(row pgx.Row) (*DbUser, error) {
	var u DbUser
	if err := row.Scan(&u.ID, &u.OwnerLogin, &u.DbType, &u.DbName, &u.Username, &u.Host, &u.Port, &u.SizeGB, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

const dbUserColumns = `id, owner_login, db_type, db_name, username, host, port, size_gb, created_at`

func (r *Repository) Create(ctx context.Context, u *DbUser) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO database_users
		(id, owner_login, db_type, db_name, username, host, port, size_gb, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now())`,
		u.ID, u.OwnerLogin, u.DbType, u.DbName, u.Username, u.Host, u.Port, u.SizeGB)
	return err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*DbUser, error) {
	return scanDbUser(r.pool.QueryRow(ctx, `SELECT `+dbUserColumns+` FROM database_users WHERE id = $1`, id))
}

func (r *Repository) ListByOwner(ctx context.Context, owner string) ([]DbUser, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+dbUserColumns+` FROM database_users WHERE owner_login = $1 ORDER BY created_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DbUser{}
	for rows.Next() {
		u, err := scanDbUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	if out == nil {
		out = []DbUser{}
	}
	return out, rows.Err()
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM database_users WHERE id = $1`, id)
	return err
}

// DbUsageGB — занятое место в СУБД пользователя (учитывается в лимите тарифа).
func (r *Repository) DbUsageGB(ctx context.Context, owner string) float64 {
	var total float64
	_ = r.pool.QueryRow(ctx, `SELECT coalesce(sum(size_gb), 0) FROM database_users WHERE owner_login = $1`, owner).Scan(&total)
	return total
}

// StorageUsageGB — всё занятое место: контейнеры + файлы + базы.
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
