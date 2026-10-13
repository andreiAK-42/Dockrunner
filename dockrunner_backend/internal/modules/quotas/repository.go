package quotas

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/admin"
)

type Quotas struct {
	AppsUsed   int     `json:"apps_used"`
	AppsLimit  int     `json:"apps_limit"`
	PortsUsed  int     `json:"ports_used"`
	PortsLimit int     `json:"ports_limit"`
	PortsMin   int     `json:"ports_min"`
	PortsMax   int     `json:"ports_max"`
	EnvUsed    int     `json:"env_used"`
	EnvLimit   int     `json:"env_limit"`
	StorageGB  float64 `json:"storage_gb_used"`
	StorageLim float64 `json:"storage_gb_limit"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

func (r *Repository) userPlan(ctx context.Context, owner string) string {
	var plan string
	if err := r.pool.QueryRow(ctx, `SELECT plan_id FROM users WHERE login = $1`, owner).Scan(&plan); err != nil || plan == "" {
		return "student"
	}
	return plan
}

func (r *Repository) appsUsed(ctx context.Context, owner string) int {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT count(DISTINCT app_name) FROM containers WHERE owner_login = $1`, owner).Scan(&total)
	return total
}

func (r *Repository) portsUsed(ctx context.Context, owner string) int {
	rows, err := r.pool.Query(ctx, `SELECT ports FROM containers WHERE owner_login = $1`, owner)
	if err != nil {
		return 0
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var raw string
		var ports []admin.PortMapping
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(raw), &ports); err != nil {
			continue
		}
		total += len(ports)
	}
	return total
}

func (r *Repository) envUsed(ctx context.Context, owner string) int {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT count(*) FROM env_configs WHERE owner_login = $1`, owner).Scan(&total)
	return total
}

func (r *Repository) storageUsed(ctx context.Context, owner string) float64 {
	var containers, files float64
	_ = r.pool.QueryRow(ctx, `SELECT coalesce(sum(containers_gb), 0), coalesce(sum(files_gb), 0)
		FROM storage_usage WHERE owner_login = $1`, owner).Scan(&containers, &files)
	var databases float64
	_ = r.pool.QueryRow(ctx, `SELECT coalesce(sum(size_gb), 0)
		FROM database_users WHERE owner_login = $1`, owner).Scan(&databases)
	return containers + files + databases
}
