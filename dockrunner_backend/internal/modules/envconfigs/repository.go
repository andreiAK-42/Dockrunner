package envconfigs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const maxPerUser = 10

type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type EnvConfig struct {
	ID          uuid.UUID `json:"id"`
	OwnerLogin  string    `json:"owner_login"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Vars        []EnvVar  `json:"vars"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

func scanConfig(row pgx.Row) (*EnvConfig, error) {
	var c EnvConfig
	var vars string
	if err := row.Scan(&c.ID, &c.OwnerLogin, &c.Name, &c.Description, &vars, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	c.Vars = decodeVars(vars)
	return &c, nil
}

func decodeVars(raw string) []EnvVar {
	vars := []EnvVar{}
	if raw == "" {
		return vars
	}
	_ = json.Unmarshal([]byte(raw), &vars)
	return vars
}

const configColumns = `id, owner_login, name, description, vars, created_at, updated_at`

func (r *Repository) ListByOwner(ctx context.Context, owner string) ([]EnvConfig, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+configColumns+` FROM env_configs WHERE owner_login = $1 ORDER BY updated_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnvConfig{}
	for rows.Next() {
		var c EnvConfig
		var vars string
		if err := rows.Scan(&c.ID, &c.OwnerLogin, &c.Name, &c.Description, &vars, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Vars = decodeVars(vars)
		out = append(out, c)
	}
	if out == nil {
		out = []EnvConfig{}
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, owner, id string) (*EnvConfig, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	row := r.pool.QueryRow(ctx, `SELECT `+configColumns+` FROM env_configs WHERE id = $1 AND owner_login = $2`, uid, owner)
	return scanConfig(row)
}

func (r *Repository) CountByOwner(ctx context.Context, owner string) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM env_configs WHERE owner_login = $1`, owner).Scan(&total)
	return total, err
}

func (r *Repository) Create(ctx context.Context, owner, name, description string, vars []EnvVar) (*EnvConfig, error) {
	raw, _ := json.Marshal(vars)
	id := uuid.New()
	row := r.pool.QueryRow(ctx, `INSERT INTO env_configs (id, owner_login, name, description, vars)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+configColumns, id, owner, name, description, string(raw))
	return scanConfig(row)
}

func (r *Repository) Update(ctx context.Context, owner, id, name, description string, vars []EnvVar) (*EnvConfig, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(vars)
	row := r.pool.QueryRow(ctx, `UPDATE env_configs
		SET name = $3, description = $4, vars = $5, updated_at = now()
		WHERE id = $1 AND owner_login = $2
		RETURNING `+configColumns, uid, owner, name, description, string(raw))
	return scanConfig(row)
}

func (r *Repository) Delete(ctx context.Context, owner, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `DELETE FROM env_configs WHERE id = $1 AND owner_login = $2`, uid, owner)
	return err
}
