package volumes

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Volume struct {
	NodeID        string    `json:"node_id" db:"node_id"`
	Name          string    `json:"name" db:"name"`
	OwnerLogin    string    `json:"owner_login" db:"owner_login"`
	ContainerPath string    `json:"container_path" db:"container_path"`
	SizeGB        float64   `json:"size_gb" db:"size_gb"`
	Status        string    `json:"status" db:"status"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

const volumeColumns = `node_id, name, owner_login, container_path, size_gb, status, updated_at`

func (r *Repository) ListByOwner(ctx context.Context, owner string) ([]Volume, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+volumeColumns+` FROM volumes WHERE owner_login = $1 ORDER BY updated_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Volume{}
	for rows.Next() {
		var v Volume
		if err := rows.Scan(&v.NodeID, &v.Name, &v.OwnerLogin, &v.ContainerPath, &v.SizeGB, &v.Status, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if out == nil {
		out = []Volume{}
	}
	return out, rows.Err()
}

func (r *Repository) Upsert(ctx context.Context, v Volume) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO volumes
		(node_id, name, owner_login, container_path, size_gb, status, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (node_id, name) DO UPDATE SET
		owner_login=excluded.owner_login, container_path=excluded.container_path,
		status=excluded.status, updated_at=now()`,
		v.NodeID, v.Name, v.OwnerLogin, v.ContainerPath, v.SizeGB, v.Status)
	return err
}

func (r *Repository) Delete(ctx context.Context, owner, node, name string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM volumes WHERE owner_login = $1 AND node_id = $2 AND name = $3`, owner, node, name)
	return err
}
