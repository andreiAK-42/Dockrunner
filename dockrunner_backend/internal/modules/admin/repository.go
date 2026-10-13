package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type PortMapping struct {
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
}

type ContainerRecord struct {
	ID         string        `json:"id"`
	AppID      string        `json:"app_id"`
	AppName    string        `json:"app_name"`
	OwnerLogin string        `json:"owner_login"`
	Image      string        `json:"image"`
	Status     string        `json:"status"`
	NodeID     string        `json:"node_id"`
	Ports      []PortMapping `json:"ports"`
	CPUCores   float64       `json:"cpu_cores"`
	MemoryGB   float64       `json:"memory_gb"`
	DiskGB     float64       `json:"disk_gb"`
	ExpiresAt  *time.Time    `json:"expires_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

func scanRecord(row interface {
	Scan(dest ...any) error
}, c *ContainerRecord) error {
	var ports string
	var expiresAt *time.Time
	if err := row.Scan(&c.ID, &c.AppID, &c.AppName, &c.OwnerLogin, &c.Image, &c.Status, &c.NodeID, &ports, &c.CPUCores, &c.MemoryGB, &c.DiskGB, &expiresAt, &c.UpdatedAt); err != nil {
		return err
	}
	c.Ports = []PortMapping{}
	if ports != "" {
		_ = json.Unmarshal([]byte(ports), &c.Ports)
	}
	c.ExpiresAt = expiresAt
	return nil
}

const containerColumns = `id, app_id, app_name, owner_login, image, status, node_id, ports, cpu_cores, memory_gb, disk_gb, expires_at, updated_at`

func collectRecords(rows pgx.Rows) ([]ContainerRecord, error) {
	defer rows.Close()
	out := []ContainerRecord{}
	for rows.Next() {
		var c ContainerRecord
		if err := scanRecord(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []ContainerRecord{}
	}
	return out, rows.Err()
}

func (r *Repository) ListContainers(ctx context.Context) ([]ContainerRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+containerColumns+` FROM containers ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	return collectRecords(rows)
}

func (r *Repository) GetContainer(ctx context.Context, id string) (*ContainerRecord, error) {
	var c ContainerRecord
	err := scanRecord(r.pool.QueryRow(ctx, `SELECT `+containerColumns+` FROM containers WHERE id = $1`, id), &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListByNode(ctx context.Context, nodeID string) ([]ContainerRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+containerColumns+` FROM containers WHERE node_id = $1 ORDER BY updated_at DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	return collectRecords(rows)
}

func (r *Repository) ListByOwner(ctx context.Context, owner string) ([]ContainerRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+containerColumns+` FROM containers WHERE owner_login = $1 ORDER BY updated_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	return collectRecords(rows)
}

func (r *Repository) FindLatestByApp(ctx context.Context, owner, appName string) (*ContainerRecord, error) {
	var c ContainerRecord
	err := scanRecord(r.pool.QueryRow(ctx, `SELECT `+containerColumns+` FROM containers WHERE owner_login = $1 AND app_name = $2
		ORDER BY updated_at DESC LIMIT 1`, owner, appName), &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) UpsertContainer(ctx context.Context, c ContainerRecord) error {
	raw, _ := json.Marshal(c.Ports)
	_, err := r.pool.Exec(ctx, `INSERT INTO containers
		(id, app_id, app_name, owner_login, image, status, node_id, ports, cpu_cores, memory_gb, disk_gb, expires_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now())
		ON CONFLICT (id) DO UPDATE SET
		app_id=excluded.app_id, app_name=excluded.app_name, owner_login=excluded.owner_login,
		image=excluded.image, status=excluded.status, node_id=excluded.node_id,
		ports=excluded.ports, cpu_cores=excluded.cpu_cores, memory_gb=excluded.memory_gb, disk_gb=excluded.disk_gb, expires_at=excluded.expires_at, updated_at=now()`,
		c.ID, c.AppID, c.AppName, c.OwnerLogin, c.Image, c.Status, c.NodeID, string(raw), c.CPUCores, c.MemoryGB, c.DiskGB, c.ExpiresAt)
	return err
}

func (r *Repository) SetContainerStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE containers SET status = $1, updated_at = now() WHERE id = $2`, status, id)
	return err
}

func (r *Repository) DeleteContainer(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM containers WHERE id = $1`, id)
	return err
}
