package pricing

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Rates — почасовые расценки агента в рублях.
type Rates struct {
	AgentID        string    `json:"agent_id" db:"agent_id"`
	CpuPerCoreHour float64   `json:"cpu_per_core_hour" db:"cpu_per_core_hour"`
	MemPerGBHour   float64   `json:"mem_per_gb_hour" db:"mem_per_gb_hour"`
	DiskPerGBHour  float64   `json:"disk_per_gb_hour" db:"disk_per_gb_hour"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

const rateColumns = `agent_id, cpu_per_core_hour, mem_per_gb_hour, disk_per_gb_hour, updated_at`

func (r *Repository) List(ctx context.Context) ([]Rates, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+rateColumns+` FROM agent_pricing ORDER BY agent_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rates{}
	for rows.Next() {
		var rates Rates
		if err := rows.Scan(&rates.AgentID, &rates.CpuPerCoreHour, &rates.MemPerGBHour, &rates.DiskPerGBHour, &rates.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rates)
	}
	if out == nil {
		out = []Rates{}
	}
	return out, rows.Err()
}

// GetFor возвращает тарифы узла. Строки может не быть — тогда всё по нулям (бесплатно).
func (r *Repository) GetFor(ctx context.Context, agentID string) (Rates, error) {
	var rates Rates
	err := r.pool.QueryRow(ctx, `SELECT `+rateColumns+` FROM agent_pricing WHERE agent_id = $1`, agentID).
		Scan(&rates.AgentID, &rates.CpuPerCoreHour, &rates.MemPerGBHour, &rates.DiskPerGBHour, &rates.UpdatedAt)
	if err != nil {
		return Rates{AgentID: agentID}, nil
	}
	return rates, nil
}

func (r *Repository) Upsert(ctx context.Context, rates Rates) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO agent_pricing
		(agent_id, cpu_per_core_hour, mem_per_gb_hour, disk_per_gb_hour, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (agent_id) DO UPDATE SET
		cpu_per_core_hour = excluded.cpu_per_core_hour,
		mem_per_gb_hour = excluded.mem_per_gb_hour,
		disk_per_gb_hour = excluded.disk_per_gb_hour,
		updated_at = now()`,
		rates.AgentID, rates.CpuPerCoreHour, rates.MemPerGBHour, rates.DiskPerGBHour)
	return err
}
