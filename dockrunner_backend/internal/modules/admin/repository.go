package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type PortMapping struct {
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
}

type VolumeSpec struct {
	Name          string `json:"name"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
}

type ContainerRecord struct {
	ID            string        `json:"id"`
	AppID         string        `json:"app_id"`
	AppName       string        `json:"app_name"`
	OwnerLogin    string        `json:"owner_login"`
	Image         string        `json:"image"`
	Status        string        `json:"status"`
	NodeID        string        `json:"node_id"`
	NodeIP        string        `json:"node_ip,omitempty"`
	Ports         []PortMapping `json:"ports"`
	Volumes       []VolumeSpec  `json:"volumes"`
	Env           map[string]string `json:"env"`
	CPUCores      float64       `json:"cpu_cores"`
	MemoryGB      float64       `json:"memory_gb"`
	DiskGB        float64       `json:"disk_gb"`
	ExpiresAt     *time.Time    `json:"expires_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	RestartPolicy string        `json:"restart_policy,omitempty"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

// Pool returns the underlying database pool.
func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

func scanRecord(row interface {
	Scan(dest ...any) error
}, c *ContainerRecord) error {
	var ports, volumes, env, restartPolicy string
	var expiresAt *time.Time
	if err := row.Scan(&c.ID, &c.AppID, &c.AppName, &c.OwnerLogin, &c.Image, &c.Status, &c.NodeID, &c.NodeIP, &ports, &volumes, &env, &c.CPUCores, &c.MemoryGB, &c.DiskGB, &restartPolicy, &expiresAt, &c.UpdatedAt); err != nil {
		return err
	}
	c.Ports = []PortMapping{}
	if ports != "" {
		_ = json.Unmarshal([]byte(ports), &c.Ports)
	}
	c.Volumes = []VolumeSpec{}
	if volumes != "" {
		_ = json.Unmarshal([]byte(volumes), &c.Volumes)
	}
	c.Env = map[string]string{}
	if env != "" {
		_ = json.Unmarshal([]byte(env), &c.Env)
	}
	c.RestartPolicy = restartPolicy
	c.ExpiresAt = expiresAt
	return nil
}

const containerColumns = `c.id, c.app_id, c.app_name, c.owner_login, c.image, c.status, c.node_id, coalesce(a.ip, '') as node_ip, c.ports, c.volumes, c.env, c.cpu_cores, c.memory_gb, c.disk_gb, c.restart_policy, c.expires_at, c.updated_at`

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
	rows, err := r.pool.Query(ctx, `SELECT `+containerColumns+` FROM containers c LEFT JOIN agents a ON c.node_id = a.id ORDER BY c.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	return collectRecords(rows)
}

func (r *Repository) GetContainer(ctx context.Context, id string) (*ContainerRecord, error) {
	var c ContainerRecord
	err := scanRecord(r.pool.QueryRow(ctx, `SELECT `+containerColumns+` FROM containers c LEFT JOIN agents a ON c.node_id = a.id WHERE c.id = $1`, id), &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListByNode(ctx context.Context, nodeID string) ([]ContainerRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+containerColumns+` FROM containers c LEFT JOIN agents a ON c.node_id = a.id WHERE c.node_id = $1 ORDER BY c.updated_at DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	return collectRecords(rows)
}

func (r *Repository) ListByOwner(ctx context.Context, owner string) ([]ContainerRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+containerColumns+` FROM containers c LEFT JOIN agents a ON c.node_id = a.id WHERE c.owner_login = $1 ORDER BY c.updated_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	return collectRecords(rows)
}

func (r *Repository) FindLatestByApp(ctx context.Context, owner, appName string) (*ContainerRecord, error) {
	var c ContainerRecord
	err := scanRecord(r.pool.QueryRow(ctx, `SELECT `+containerColumns+` FROM containers c LEFT JOIN agents a ON c.node_id = a.id WHERE c.owner_login = $1 AND c.app_name = $2
		ORDER BY c.updated_at DESC LIMIT 1`, owner, appName), &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) UpsertContainer(ctx context.Context, c ContainerRecord) error {
	rawPorts, _ := json.Marshal(c.Ports)
	rawVolumes, _ := json.Marshal(c.Volumes)
	rawEnv, _ := json.Marshal(c.Env)
	_, err := r.pool.Exec(ctx, `INSERT INTO containers
		(id, app_id, app_name, owner_login, image, status, node_id, ports, volumes, env, cpu_cores, memory_gb, disk_gb, restart_policy, expires_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,now())
		ON CONFLICT (id) DO UPDATE SET
		app_id=excluded.app_id, app_name=excluded.app_name, owner_login=excluded.owner_login,
		image=excluded.image, status=excluded.status, node_id=excluded.node_id,
		ports=excluded.ports, volumes=excluded.volumes, env=excluded.env, cpu_cores=excluded.cpu_cores, memory_gb=excluded.memory_gb, disk_gb=excluded.disk_gb, restart_policy=excluded.restart_policy, expires_at=excluded.expires_at, updated_at=now()`,
		c.ID, c.AppID, c.AppName, c.OwnerLogin, c.Image, c.Status, c.NodeID, string(rawPorts), string(rawVolumes), string(rawEnv), c.CPUCores, c.MemoryGB, c.DiskGB, c.RestartPolicy, c.ExpiresAt)
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

// UpdateContainerParams updates container parameters (image, env, limits, ports, volumes, etc.)
// Used by admin to modify container launch parameters.
func (r *Repository) UpdateContainerParams(ctx context.Context, id string, params map[string]any) error {
	setParts := []string{"updated_at = now()"}
	args := []any{id}
	argIdx := 2

	// Fields that need JSON marshaling
	jsonFields := map[string]bool{
		"ports":   true,
		"volumes": true,
		"env":     true,
	}

	// Allowed fields that admin can update
	allowedFields := map[string]string{
		"image":         "image",
		"cpu_cores":     "cpu_cores",
		"memory_gb":     "memory_gb",
		"disk_gb":       "disk_gb",
		"ports":         "ports",
		"env":           "env",
		"volumes":       "volumes",
		"restart_policy": "restart_policy",
		"expires_at":    "expires_at",
	}

	for key, dbCol := range allowedFields {
		if val, ok := params[key]; ok && val != nil {
			// Marshal JSON fields
			if jsonFields[key] {
				jsonBytes, err := json.Marshal(val)
				if err != nil {
					return fmt.Errorf("failed to marshal %s: %w", key, err)
				}
				val = string(jsonBytes)
			}
			setParts = append(setParts, fmt.Sprintf("%s = $%d", dbCol, argIdx))
			args = append(args, val)
			argIdx++
		}
	}

	if len(setParts) == 1 { // only updated_at
		return nil
	}

	query := fmt.Sprintf("UPDATE containers SET %s WHERE id = $1", strings.Join(setParts, ", "))
	_, err := r.pool.Exec(ctx, query, args...)
	return err
}
