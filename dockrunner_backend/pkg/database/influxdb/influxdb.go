package influxdb

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"go.uber.org/zap"
)

type Config struct {
	URL     string
	Token   string
	Org     string
	Bucket  string
	Timeout time.Duration
}

type Client struct {
	client influxdb2.Client
	org    string
	bucket string
}

func New(ctx context.Context, cfg Config, logger *zap.Logger) (*Client, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client := influxdb2.NewClientWithOptions(
		cfg.URL,
		cfg.Token,
		influxdb2.DefaultOptions().SetHTTPRequestTimeout(uint(timeout.Seconds())),
	)

	if err := waitForReady(ctx, client, timeout); err != nil {
		client.Close()
		return nil, err
	}

	logger.Info(
		"influxdb connected",
		zap.String("url", cfg.URL),
		zap.String("bucket", cfg.Bucket),
	)

	return &Client{
		client: client,
		org:    cfg.Org,
		bucket: cfg.Bucket,
	}, nil
}

func waitForReady(ctx context.Context, client influxdb2.Client, timeout time.Duration) error {
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ready, err := client.Ready(readyCtx)
	if err != nil {
		return fmt.Errorf("influxdb not ready: %w", err)
	}

	if !isReadyStatus(ready) {
		return fmt.Errorf("influxdb not ready: server is not ready")
	}

	return nil
}

func isReadyStatus(ready *domain.Ready) bool {
	if ready == nil {
		return false
	}
	if ready.Status == nil {
		return false
	}
	return string(*ready.Status) == "ready"
}

func (c *Client) Close() {
	c.client.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	ready, err := c.client.Ready(ctx)
	if err != nil {
		return err
	}

	if !isReadyStatus(ready) {
		return fmt.Errorf("server not ready")
	}

	return nil
}

type ServerMetrics struct {
	ServerID string
	CPU      float64
	MemUsed  float64
	MemTotal float64
}

func (c *Client) WriteServerMetrics(ctx context.Context, metrics ServerMetrics) error {
	memPercent := memUsagePercent(metrics.MemUsed, metrics.MemTotal)

	point := influxdb2.NewPointWithMeasurement("server_metrics").
		AddTag("server_id", metrics.ServerID).
		AddField("cpu", metrics.CPU).
		AddField("mem", memPercent).
		SetTime(time.Now())

	writeAPI := c.client.WriteAPIBlocking(c.org, c.bucket)

	return writeAPI.WritePoint(ctx, point)
}

func memUsagePercent(used, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return used / total * 100
}

type ContainerMetrics struct {
	ContainerID string
	CPU         float64
	Memory      float64
}

func (c *Client) WriteContainerMetrics(ctx context.Context, metrics ContainerMetrics) error {
	point := influxdb2.NewPointWithMeasurement("container_metrics").
		AddTag("container_id", metrics.ContainerID).
		AddField("cpu", metrics.CPU).
		AddField("memory", metrics.Memory).
		SetTime(time.Now())

	writeAPI := c.client.WriteAPIBlocking(c.org, c.bucket)

	return writeAPI.WritePoint(ctx, point)
}


type MetricPoint struct {
	Time   time.Time
	Fields map[string]any
}

func (c *Client) QueryRange(
	ctx context.Context,
	measurement string,
	idTag string,
	id string,
	since time.Duration,
) ([]MetricPoint, error) {
	flux := buildRangeQuery(c.bucket, measurement, idTag, id, since)

	result, err := c.client.QueryAPI(c.org).Query(ctx, flux)
	if err != nil {
		return nil, err
	}

	return readTable(result)
}

func buildRangeQuery(
	bucket string,
	measurement string,
	idTag string,
	id string,
	since time.Duration,
) string {
	return fmt.Sprintf(`from(bucket: %q)
		|> range(start: -%s)
		|> filter(fn: (r) => r["_measurement"] == %q and r[%q] == %q)
		|> aggregateWindow(every: 5m, fn: mean, createEmpty: false)`,
		bucket,
		since.String(),
		measurement,
		idTag,
		id,
	)
}

func readTable(result *api.QueryTableResult) ([]MetricPoint, error) {
	points := make([]MetricPoint, 0)

	for result.Next() {
		record := result.Record()

		points = append(points, MetricPoint{
			Time:   record.Time(),
			Fields: record.Values(),
		})
	}

	return points, result.Err()
}

type DailyPoint struct {
	Day string  `json:"day"`
	CPU float64 `json:"cpu"`
	RAM float64 `json:"ram"`
}

func (c *Client) QueryClusterDaily(ctx context.Context, days int) ([]DailyPoint, error) {
	days = normalizeDays(days)
	flux := fmt.Sprintf(`from(bucket: %q)
		|> range(start: -%dd)
		|> filter(fn: (r) => r["_measurement"] == "server_metrics")
		|> aggregateWindow(every: 1d, fn: mean, createEmpty: false)
		|> group(columns: ["_time", "_field"])
		|> mean(column: "_value")`,
		c.bucket, days)
	result, err := c.client.QueryAPI(c.org).Query(ctx, flux)
	if err != nil {
		return nil, err
	}
	return collectDaily(result)
}

func normalizeDays(days int) int {
	if days <= 0 || days > 90 {
		return 14
	}
	return days
}

func collectDaily(result *api.QueryTableResult) ([]DailyPoint, error) {
	byDay := map[string]*DailyPoint{}
	order := []string{}
	for result.Next() {
		record := result.Record()
		day := record.Time().Format("02.01")
		point, found := byDay[day]
		if !found {
			point = &DailyPoint{Day: day}
			byDay[day] = point
			order = append(order, day)
		}
		value, _ := record.Value().(float64)
		if record.Field() == "cpu" {
			point.CPU = value
		} else if record.Field() == "mem" {
			point.RAM = value
		}
	}
	if err := result.Err(); err != nil {
		return nil, err
	}
	out := make([]DailyPoint, 0, len(order))
	for _, day := range order {
		out = append(out, *byDay[day])
	}
	return out, nil
}
