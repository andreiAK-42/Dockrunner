package deploymentrequests

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
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

type Request struct {
	ID                uuid.UUID         `json:"id"`
	OwnerLogin        string            `json:"ownerLogin"`
	AppName           string            `json:"appName"`
	RepoURL           string            `json:"repoUrl"`
	Image             string            `json:"image"`
	ContainerName     string            `json:"containerName"`
	NodeID            string            `json:"nodeId"`
	Ports             []PortMapping     `json:"ports"`
	Volumes           []VolumeSpec      `json:"volumes"`
	Env               map[string]string `json:"env"`
	CPUCores          float64           `json:"cpuCores"`
	MemoryGB          float64           `json:"memoryGB"`
	DiskGB            float64           `json:"diskGB"`
	DurationHours     float64           `json:"durationHours"`
	// Branch — git branch to deploy (required when repo_url is set)
	Branch            string            `json:"branch"`
	// Services — выбранные сервисы compose.
	BuildMode         string            `json:"buildMode"`
	// Services — сервисы compose (пусто = один контейнер).
	// Цена заявки = цена одного контейнера × число сервисов.
	Services          []string          `json:"services"`
	Price             float64           `json:"price"`
	Status            string            `json:"status"`
	Comment           string            `json:"comment"`
	DecidedBy         string            `json:"decidedBy"`
	DecidedAt         *time.Time        `json:"decidedAt"`
	DeploymentID      string            `json:"deploymentId"`
	// Type — тип заявки: "create" (новый деплой) или "update" (изменение существующего)
	Type              string            `json:"type"`
	// ExistingDeploymentID — ID существующего деплоя для обновления
	ExistingDeploymentID string         `json:"existingDeploymentId"`
	// RestartPolicy — политика перезапуска контейнера
	RestartPolicy     string            `json:"restartPolicy"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

const requestColumns = `id, owner_login, app_name, repo_url, image, container_name, node_id, ports, volumes, env, cpu_cores, memory_gb, disk_gb, duration_hours, branch, services, build_mode, price, status, comment, decided_by, decided_at, deployment_id, type, existing_deployment_id, restart_policy, created_at, updated_at`

func scanRequest(row pgx.Row) (*Request, error) {
	var r Request
	var portsJSON, volumesJSON, envJSON, servicesJSON string
	var reqType, existingDeploymentID, restartPolicy string
	err := row.Scan(
		&r.ID, &r.OwnerLogin, &r.AppName, &r.RepoURL, &r.Image, &r.ContainerName, &r.NodeID,
		&portsJSON, &volumesJSON, &envJSON,
		&r.CPUCores, &r.MemoryGB, &r.DiskGB, &r.DurationHours, &r.Branch, &servicesJSON, &r.BuildMode, &r.Price,
		&r.Status, &r.Comment, &r.DecidedBy, &r.DecidedAt, &r.DeploymentID,
		&reqType, &existingDeploymentID, &restartPolicy,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(portsJSON), &r.Ports)
	_ = json.Unmarshal([]byte(volumesJSON), &r.Volumes)
	_ = json.Unmarshal([]byte(envJSON), &r.Env)
	_ = json.Unmarshal([]byte(servicesJSON), &r.Services)
	if r.Ports == nil {
		r.Ports = []PortMapping{}
	}
	if r.Volumes == nil {
		r.Volumes = []VolumeSpec{}
	}
	if r.Env == nil {
		r.Env = map[string]string{}
	}
	if r.Services == nil {
		r.Services = []string{}
	}
	r.Type = reqType
	r.ExistingDeploymentID = existingDeploymentID
	r.RestartPolicy = restartPolicy
	return &r, nil
}

func (r *Repository) Create(ctx context.Context, req *Request) error {
	portsJSON, _ := json.Marshal(req.Ports)
	volumesJSON, _ := json.Marshal(req.Volumes)
	envJSON, _ := json.Marshal(req.Env)
	servicesJSON, _ := json.Marshal(req.Services)
	// Default type to "create" if not set
	reqType := req.Type
	if reqType == "" {
		reqType = "create"
	}
	// Default restart_policy if not set
	restartPolicy := req.RestartPolicy
	if restartPolicy == "" {
		restartPolicy = "unless-stopped"
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO launch_requests (id, owner_login, app_name, repo_url, image, container_name, node_id, ports, volumes, env, cpu_cores, memory_gb, disk_gb, duration_hours, branch, services, build_mode, price, status, type, existing_deployment_id, restart_policy, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,now(),now())`,
		req.ID, req.OwnerLogin, req.AppName, req.RepoURL, req.Image, req.ContainerName, req.NodeID,
		string(portsJSON), string(volumesJSON), string(envJSON),
		req.CPUCores, req.MemoryGB, req.DiskGB, req.DurationHours, req.Branch, string(servicesJSON), req.BuildMode, req.Price,
		req.Status, reqType, req.ExistingDeploymentID, restartPolicy,
	)
	return err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Request, error) {
	return scanRequest(r.pool.QueryRow(ctx, `SELECT `+requestColumns+` FROM launch_requests WHERE id = $1`, id))
}

func (r *Repository) ListByOwner(ctx context.Context, owner string) ([]Request, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+requestColumns+` FROM launch_requests WHERE owner_login = $1 ORDER BY created_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.collect(rows)
}

func (r *Repository) ListPending(ctx context.Context) ([]Request, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+requestColumns+` FROM launch_requests WHERE status = 'pending' ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.collect(rows)
}

func (r *Repository) ListAll(ctx context.Context) ([]Request, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+requestColumns+` FROM launch_requests ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.collect(rows)
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status, comment, decidedBy string, deploymentID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE launch_requests SET status = $1, comment = $2, decided_by = $3, decided_at = now(), deployment_id = $4, updated_at = now()
		WHERE id = $5`, status, comment, decidedBy, deploymentID, id)
	return err
}

func (r *Repository) UpdatePrice(ctx context.Context, id uuid.UUID, price float64) error {
	_, err := r.pool.Exec(ctx, `UPDATE launch_requests SET price = $1, updated_at = now() WHERE id = $2`, price, id)
	return err
}

func (r *Repository) collect(rows pgx.Rows) ([]Request, error) {
	out := []Request{}
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *req)
	}
	if out == nil {
		out = []Request{}
	}
	return out, rows.Err()
}
