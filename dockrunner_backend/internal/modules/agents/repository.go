package agents

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ErrNoAPIURL means the agent never reported its HTTP address
// (AGENT_PUBLIC_URL is not set on the agent side).
var ErrNoAPIURL = errors.New("agent has no api url")

type Agent struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	IP          string    `json:"ip" db:"ip"`
	City        string    `json:"city" db:"city"`
	Lat         float64   `json:"lat" db:"lat"`
	Lng         float64   `json:"lng" db:"lng"`
	ApiURL      string    `json:"api_url" db:"api_url"`
	CPUTotal    int       `json:"cpu_total" db:"cpu_total"`
	CPUPercent  float64   `json:"cpu_percent" db:"cpu_percent"`
	CPUMhz      int       `json:"cpu_mhz" db:"cpu_mhz"`
	MemUsedGB   float64   `json:"mem_used_gb" db:"mem_used_gb"`
	MemTotalGB  float64   `json:"mem_total_gb" db:"mem_total_gb"`
	MemType     string    `json:"mem_type" db:"mem_type"`
	MemSpeedMhz int       `json:"mem_speed_mhz" db:"mem_speed_mhz"`
	DiskUsedGB  float64   `json:"disk_used_gb" db:"disk_used_gb"`
	DiskTotalGB float64   `json:"disk_total_gb" db:"disk_total_gb"`
	DiskType    string    `json:"disk_type" db:"disk_type"`
	Containers  int       `json:"containers" db:"containers"`
	Description string    `json:"description" db:"description"`
	NetDownMbps float64   `json:"net_down_mbps" db:"net_down_mbps"`
	NetUpMbps   float64   `json:"net_up_mbps" db:"net_up_mbps"`
	LastSeen    time.Time `json:"last_seen" db:"last_seen"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

func (r *Repository) UpsertHeartbeat(ctx context.Context, a Agent) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO agents
		(id, name, ip, city, lat, lng, api_url, cpu_total, cpu_percent, cpu_mhz, mem_used_gb, mem_total_gb, mem_type, mem_speed_mhz, disk_used_gb, disk_total_gb, disk_type, containers, description, net_down_mbps, net_up_mbps, last_seen)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		ON CONFLICT (id) DO UPDATE SET
		name=excluded.name, ip=CASE WHEN excluded.ip = '' THEN agents.ip ELSE excluded.ip END, city=excluded.city, lat=excluded.lat, lng=excluded.lng,
		api_url=excluded.api_url, cpu_total=excluded.cpu_total, cpu_percent=excluded.cpu_percent, cpu_mhz=excluded.cpu_mhz,
		mem_used_gb=excluded.mem_used_gb, mem_total_gb=excluded.mem_total_gb, mem_type=excluded.mem_type, mem_speed_mhz=excluded.mem_speed_mhz,
		disk_used_gb=excluded.disk_used_gb, disk_total_gb=excluded.disk_total_gb, disk_type=excluded.disk_type,
		containers=excluded.containers,
		description=CASE WHEN excluded.description = '' THEN agents.description ELSE excluded.description END,
		net_down_mbps=excluded.net_down_mbps, net_up_mbps=excluded.net_up_mbps,
		last_seen=excluded.last_seen`,
		a.ID, a.Name, a.IP, a.City, a.Lat, a.Lng, a.ApiURL, a.CPUTotal, a.CPUPercent, a.CPUMhz,
		a.MemUsedGB, a.MemTotalGB, a.MemType, a.MemSpeedMhz,
		a.DiskUsedGB, a.DiskTotalGB, a.DiskType,
		a.Containers, a.Description, a.NetDownMbps, a.NetUpMbps, a.LastSeen)
	return err
}

// EnsureAgentIdentity переносит узел на новый id, если агент сменил
// идентификатор: например, раньше присылал AGENT_ID=worker-01 и
// AGENT_NAME=Сатурн, а теперь только AGENT_NAME=Сатурн. Без переноса в
// containers/volumes/storage_usage остался бы старый node_id, и центр считал
// бы узел офлайн, а приложения — привязанными к несуществующему воркеру.
//
// Узел ищем по имени (если heartbeat был недавно) или по api_url
// (AGENT_PUBLIC_URL) — адрес API однозначно указывает на тот же воркер,
// поэтому по нему переносим даже после долгого простоя.
// Переносим только когда строки с новым id ещё нет.
func (r *Repository) EnsureAgentIdentity(ctx context.Context, id, name, apiURL string) error {
	if id == "" || name == "" {
		return nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1)`, id).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var oldID string
	err = tx.QueryRow(ctx, `SELECT id FROM agents
		WHERE id <> $2
		  AND (
		    (name = $1 AND last_seen > now() - interval '15 minutes')
		    OR ($3 <> '' AND api_url = $3)
		  )
		ORDER BY (name = $1) DESC, last_seen DESC
		LIMIT 1 FOR UPDATE`, name, id, apiURL).Scan(&oldID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	// Ссылки на узел в остальных таблицах (внешних ключей нет).
	for _, q := range []string{
		`UPDATE containers SET node_id = $2 WHERE node_id = $1`,
		`UPDATE volumes SET node_id = $2 WHERE node_id = $1`,
		`UPDATE storage_usage SET agent_id = $2 WHERE agent_id = $1`,
		`UPDATE launch_requests SET node_id = $2 WHERE node_id = $1`,
		`UPDATE agent_pricing SET agent_id = $2 WHERE agent_id = $1`,
		`UPDATE agents SET id = $2 WHERE id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, oldID, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	r.logger.Info("agent identity migrated", zap.String("from", oldID), zap.String("to", id), zap.String("name", name))
	return nil
}

func (r *Repository) List(ctx context.Context) ([]Agent, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, ip, city, lat, lng, api_url, cpu_total, cpu_percent, cpu_mhz, mem_used_gb, mem_total_gb, mem_type, mem_speed_mhz, disk_used_gb, disk_total_gb, disk_type, containers, description, net_down_mbps, net_up_mbps, last_seen FROM agents ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.IP, &a.City, &a.Lat, &a.Lng, &a.ApiURL, &a.CPUTotal, &a.CPUPercent, &a.CPUMhz, &a.MemUsedGB, &a.MemTotalGB, &a.MemType, &a.MemSpeedMhz, &a.DiskUsedGB, &a.DiskTotalGB, &a.DiskType, &a.Containers, &a.Description, &a.NetDownMbps, &a.NetUpMbps, &a.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type StorageRow struct {
	OwnerLogin   string  `json:"owner"`
	ContainersGB float64 `json:"containers_gb"`
	FilesGB      float64 `json:"files_gb"`
}

func (r *Repository) ReplaceStorage(ctx context.Context, agentID string, rows []StorageRow) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM storage_usage WHERE agent_id = $1`, agentID); err != nil {
		return err
	}
	for _, row := range rows {
		if row.OwnerLogin == "" {
			continue
		}
		if _, err := r.pool.Exec(ctx, `INSERT INTO storage_usage
			(agent_id, owner_login, containers_gb, files_gb, updated_at)
			VALUES ($1, $2, $3, $4, now())`,
			agentID, row.OwnerLogin, row.ContainersGB, row.FilesGB); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UserTotals(ctx context.Context, owner string) (float64, float64, error) {
	var containers, files float64
	err := r.pool.QueryRow(ctx, `SELECT coalesce(sum(containers_gb), 0), coalesce(sum(files_gb), 0)
		FROM storage_usage WHERE owner_login = $1`, owner).Scan(&containers, &files)
	if err != nil {
		return 0, 0, err
	}
	return containers, files, nil
}

func (r *Repository) AgentApiURL(ctx context.Context, agentID string) (string, error) {
	var url string
	err := r.pool.QueryRow(ctx, `SELECT api_url FROM agents WHERE id = $1`, agentID).Scan(&url)
	if err != nil {
		return "", err
	}
	if url == "" {
		return "", ErrNoAPIURL
	}
	return url, nil
}
