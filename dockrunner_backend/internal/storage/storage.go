package storage

// Storage aggregates all DB connections for DI
// Provides a single place to inject into modules if needed

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"dockrunner_backend/pkg/database/influxdb"
	"dockrunner_backend/pkg/queue"
)

type Storage struct {
	Postgres *pgxpool.Pool
	Influx   *influxdb.Client
	Queue    *queue.Client
}

func New(postgres *pgxpool.Pool, influx *influxdb.Client, queueClient *queue.Client) *Storage {
	return &Storage{
		Postgres: postgres,
		Influx:   influx,
		Queue:    queueClient,
	}
}
