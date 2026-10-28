package postgre

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdle     time.Duration
	HealthCheck     time.Duration
}

func New(ctx context.Context, cfg Config, logger *zap.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres url: %w", err)
	}

	configurePool(poolConfig, cfg)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pingPool(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	logger.Info(
		"postgres connected",
		zap.Int32("max_conns", poolConfig.MaxConns),
		zap.Int32("min_conns", poolConfig.MinConns),
	)

	return pool, nil
}

func configurePool(poolConfig *pgxpool.Config, cfg Config) {
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = cfg.MaxConns
	}

	if cfg.MinConns > 0 {
		poolConfig.MinConns = cfg.MinConns
	}

	if cfg.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	}

	if cfg.MaxConnIdle > 0 {
		poolConfig.MaxConnIdleTime = cfg.MaxConnIdle
	}

	if cfg.HealthCheck > 0 {
		poolConfig.HealthCheckPeriod = cfg.HealthCheck
	}
}

func pingPool(ctx context.Context, pool *pgxpool.Pool) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("postgres ping failed: %w", err)
	}

	return nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) error {
	createExtension(ctx, pool, logger)
	createAgentsTable(ctx, pool, logger)
	createUsersTable(ctx, pool, logger)
	createContainersTable(ctx, pool, logger)
	createStorageTable(ctx, pool, logger)
	createVolumesTable(ctx, pool, logger)
	createEnvConfigsTable(ctx, pool, logger)
	createLaunchRequestsTable(ctx, pool, logger)
	createAgentPricingTable(ctx, pool, logger)
	createBalanceTransactionsTable(ctx, pool, logger)
	migrateUserBalance(ctx, pool, logger)
	migrateUserSuperuser(ctx, pool, logger)
	migrateContainerExpiry(ctx, pool, logger)
	migrateContainerResources(ctx, pool, logger)
	migrateAgentHardware(ctx, pool, logger)
	migrateLaunchRequestServices(ctx, pool, logger)
	createDatabaseUsersTable(ctx, pool, logger)

	return nil
}

// execQuiet выполняет DDL и пишет Warn только при ошибке.
func execQuiet(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger, what, sql string, args ...any) {
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		logger.Warn(what, zap.Error(err))
	}
}

func createExtension(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create uuid-ossp extension", `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`)
}

func createAgentsTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create agents table", `
		CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL DEFAULT '',
			city TEXT NOT NULL DEFAULT '',
			lat DOUBLE PRECISION NOT NULL DEFAULT 0,
			lng DOUBLE PRECISION NOT NULL DEFAULT 0,
			api_url TEXT NOT NULL DEFAULT '',
			cpu_total INT NOT NULL DEFAULT 0,
			cpu_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
			mem_used_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
			mem_total_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
			disk_used_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
			disk_total_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
			containers INT NOT NULL DEFAULT 0,
			description TEXT NOT NULL,
			net_down_mbps DOUBLE PRECISION NOT NULL DEFAULT 0,
			net_up_mbps DOUBLE PRECISION NOT NULL DEFAULT 0,
			last_seen TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	for _, col := range []string{
		"ADD COLUMN IF NOT EXISTS ip TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS city TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS lat DOUBLE PRECISION NOT NULL DEFAULT 0",
		"ADD COLUMN IF NOT EXISTS lng DOUBLE PRECISION NOT NULL DEFAULT 0",
		"ADD COLUMN IF NOT EXISTS api_url TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS cpu_total INT NOT NULL DEFAULT 0",
		"ADD COLUMN IF NOT EXISTS net_down_mbps DOUBLE PRECISION NOT NULL DEFAULT 0",
		"ADD COLUMN IF NOT EXISTS net_up_mbps DOUBLE PRECISION NOT NULL DEFAULT 0",
	} {
		execQuiet(ctx, pool, logger, "failed to migrate agents table", `ALTER TABLE agents `+col)
	}
}

func createUsersTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create users table", `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			login TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			blocked BOOLEAN NOT NULL DEFAULT FALSE,
			plan_id TEXT NOT NULL DEFAULT 'student',
			plan_expires TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
}

func createContainersTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create containers table", `
		CREATE TABLE IF NOT EXISTS containers (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL DEFAULT '',
			app_name TEXT NOT NULL DEFAULT '',
			owner_login TEXT NOT NULL DEFAULT '',
			image TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'created',
			node_id TEXT NOT NULL DEFAULT '',
			ports TEXT NOT NULL DEFAULT '[]',
			volumes TEXT NOT NULL DEFAULT '[]',
			env TEXT NOT NULL DEFAULT '{}',
			cpu_cores DOUBLE PRECISION NOT NULL DEFAULT 0.25,
			memory_gb DOUBLE PRECISION NOT NULL DEFAULT 0.5,
			disk_gb DOUBLE PRECISION NOT NULL DEFAULT 1.0,
			restart_policy TEXT NOT NULL DEFAULT 'unless-stopped',
			expires_at TIMESTAMPTZ NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	execQuiet(ctx, pool, logger, "failed to migrate containers table", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS ports TEXT NOT NULL DEFAULT '[]'`)
	execQuiet(ctx, pool, logger, "failed to migrate containers table", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS volumes TEXT NOT NULL DEFAULT '[]'`)
	execQuiet(ctx, pool, logger, "failed to migrate containers table", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS env TEXT NOT NULL DEFAULT '{}'`)
	execQuiet(ctx, pool, logger, "failed to migrate containers table", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS cpu_cores DOUBLE PRECISION NOT NULL DEFAULT 0.25`)
	execQuiet(ctx, pool, logger, "failed to migrate containers table", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS memory_gb DOUBLE PRECISION NOT NULL DEFAULT 0.5`)
	execQuiet(ctx, pool, logger, "failed to migrate containers table", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS disk_gb DOUBLE PRECISION NOT NULL DEFAULT 1.0`)
	execQuiet(ctx, pool, logger, "failed to migrate containers table", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS restart_policy TEXT NOT NULL DEFAULT 'unless-stopped'`)
	execQuiet(ctx, pool, logger, "failed to migrate containers table", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL`)
}

func createStorageTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {

	execQuiet(ctx, pool, logger, "failed to create storage_usage table", `
		CREATE TABLE IF NOT EXISTS storage_usage (
			agent_id TEXT NOT NULL,
			owner_login TEXT NOT NULL,
			containers_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
			files_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (agent_id, owner_login)
		);
	`)
}

func createVolumesTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create volumes table", `
		CREATE TABLE IF NOT EXISTS volumes (
			node_id TEXT NOT NULL,
			name TEXT NOT NULL,
			owner_login TEXT NOT NULL DEFAULT '',
			container_path TEXT NOT NULL DEFAULT '',
			size_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'available',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (node_id, name)
		);
	`)
}

func createEnvConfigsTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create env_configs table", `
		CREATE TABLE IF NOT EXISTS env_configs (
			id UUID PRIMARY KEY,
			owner_login TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			vars TEXT NOT NULL DEFAULT '[]',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
}

func createLaunchRequestsTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create launch_requests table", `
		CREATE TABLE IF NOT EXISTS launch_requests (
			id UUID PRIMARY KEY,
			owner_login TEXT NOT NULL,
			app_name TEXT NOT NULL,
			repo_url TEXT NOT NULL DEFAULT '',
			image TEXT NOT NULL DEFAULT '',
			container_name TEXT NOT NULL DEFAULT '',
			node_id TEXT NOT NULL,
			ports TEXT NOT NULL DEFAULT '[]',
			volumes TEXT NOT NULL DEFAULT '[]',
			env TEXT NOT NULL DEFAULT '{}',
			cpu_cores DOUBLE PRECISION NOT NULL DEFAULT 1,
			memory_gb DOUBLE PRECISION NOT NULL DEFAULT 1,
			disk_gb DOUBLE PRECISION NOT NULL DEFAULT 5,
			duration_hours DOUBLE PRECISION NOT NULL DEFAULT 1,
			branch TEXT NOT NULL DEFAULT '',
			services TEXT NOT NULL DEFAULT '[]',
			build_mode TEXT NOT NULL DEFAULT 'dockerfile',
			price DOUBLE PRECISION NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			comment TEXT NOT NULL DEFAULT '',
			decided_by TEXT NOT NULL DEFAULT '',
			decided_at TIMESTAMPTZ NULL,
			deployment_id TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'create',
			existing_deployment_id TEXT NOT NULL DEFAULT '',
			restart_policy TEXT NOT NULL DEFAULT 'unless-stopped',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	// Backfill for existing databases (idempotent)
	for _, col := range []string{
		"ADD COLUMN IF NOT EXISTS branch TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS services TEXT NOT NULL DEFAULT '[]'",
		"ADD COLUMN IF NOT EXISTS build_mode TEXT NOT NULL DEFAULT 'dockerfile'",
		"ADD COLUMN IF NOT EXISTS type TEXT NOT NULL DEFAULT 'create'",
		"ADD COLUMN IF NOT EXISTS existing_deployment_id TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS restart_policy TEXT NOT NULL DEFAULT 'unless-stopped'",
	} {
		execQuiet(ctx, pool, logger, "failed to migrate launch_requests table", `ALTER TABLE launch_requests `+col)
	}
}

func createAgentPricingTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create agent_pricing table", `
		CREATE TABLE IF NOT EXISTS agent_pricing (
			agent_id TEXT PRIMARY KEY,
			cpu_per_core_hour DOUBLE PRECISION NOT NULL DEFAULT 0,
			mem_per_gb_hour DOUBLE PRECISION NOT NULL DEFAULT 0,
			disk_per_gb_hour DOUBLE PRECISION NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
}

func createBalanceTransactionsTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create balance_transactions table", `
		CREATE TABLE IF NOT EXISTS balance_transactions (
			id UUID PRIMARY KEY,
			owner_login TEXT NOT NULL,
			amount DOUBLE PRECISION NOT NULL,
			balance_after DOUBLE PRECISION NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
}

func migrateUserBalance(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to migrate users balance", `ALTER TABLE users ADD COLUMN IF NOT EXISTS balance DOUBLE PRECISION NOT NULL DEFAULT 0`)
}

func migrateUserSuperuser(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to migrate users superuser", `ALTER TABLE users ADD COLUMN IF NOT EXISTS is_superuser BOOLEAN NOT NULL DEFAULT FALSE`)
}

func migrateContainerExpiry(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to migrate containers expiry", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL`)
}

func migrateContainerResources(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to migrate containers resources", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS cpu_cores DOUBLE PRECISION NOT NULL DEFAULT 1`)
	execQuiet(ctx, pool, logger, "failed to migrate containers resources", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS memory_gb DOUBLE PRECISION NOT NULL DEFAULT 1`)
	execQuiet(ctx, pool, logger, "failed to migrate containers resources", `ALTER TABLE containers ADD COLUMN IF NOT EXISTS disk_gb DOUBLE PRECISION NOT NULL DEFAULT 5`)
}

func migrateAgentHardware(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to migrate agents hardware", `ALTER TABLE agents ADD COLUMN IF NOT EXISTS cpu_mhz INT NOT NULL DEFAULT 0`)
	execQuiet(ctx, pool, logger, "failed to migrate agents hardware", `ALTER TABLE agents ADD COLUMN IF NOT EXISTS mem_type TEXT NOT NULL DEFAULT ''`)
	execQuiet(ctx, pool, logger, "failed to migrate agents hardware", `ALTER TABLE agents ADD COLUMN IF NOT EXISTS mem_speed_mhz INT NOT NULL DEFAULT 0`)
	execQuiet(ctx, pool, logger, "failed to migrate agents hardware", `ALTER TABLE agents ADD COLUMN IF NOT EXISTS disk_type TEXT NOT NULL DEFAULT ''`)
}

func migrateLaunchRequestServices(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	// Columns are now added in createLaunchRequestsTable. Keep as no-op for existing migrations.
}

func createDatabaseUsersTable(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	execQuiet(ctx, pool, logger, "failed to create database_users table", `
		CREATE TABLE IF NOT EXISTS database_users (
			id UUID PRIMARY KEY,
			owner_login TEXT NOT NULL,
			db_type TEXT NOT NULL,
			db_name TEXT NOT NULL,
			username TEXT NOT NULL,
			host TEXT NOT NULL DEFAULT '',
			port INT NOT NULL DEFAULT 0,
			size_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE (owner_login, db_type, username)
		);
	`)
	execQuiet(ctx, pool, logger, "failed to index database_users table", `CREATE INDEX IF NOT EXISTS idx_database_users_owner ON database_users(owner_login)`)
}
