package victoria

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// escapeTag экранирует значение тега line protocol (запятые, пробелы, '=').
func escapeTag(s string) string {
	r := strings.NewReplacer(",", `\,`, " ", `\ `, "=", `\=`)
	return r.Replace(s)
}

// MetricsWriter пишет метрики в VictoriaMetrics (Influx line protocol).
type MetricsWriter struct {
	endpoint string
	token    string
	client   *http.Client
	logger   *slog.Logger
}

// NewMetricsWriter собирает writer. rawURL — базовый адрес, например
// http://victoria:8428. Пустой URL — writer nil-совместим: Write не вызывается.
func NewMetricsWriter(rawURL, token string, timeout time.Duration, logger *slog.Logger) *MetricsWriter {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &MetricsWriter{
		endpoint: strings.TrimRight(rawURL, "/") + "/write",
		token:    token,
		client:   &http.Client{Timeout: timeout},
		logger:   logger,
	}
}

// Write отправляет одну или несколько строк line protocol. Ошибка — для лога,
// ретрай не делаем: метрики дешевле потерять, чем копить backlog.
func (w *MetricsWriter) Write(ctx context.Context, lines string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint, strings.NewReader(lines))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if w.token != "" {
		req.Header.Set("Authorization", "Bearer "+w.token)
	}
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return fmt.Errorf("victoria metrics write: status %d", res.StatusCode)
	}
	return nil
}

// NodeMetricsLine формирует строку line protocol для метрик узла.
func NodeMetricsLine(agentID, hostname string, cpuPercent, memUsedGB, memTotalGB, diskUsedGB, diskTotalGB float64, containers int, netDownMbps, netUpMbps float64) string {
	memPercent := 0.0
	if memTotalGB > 0 {
		memPercent = memUsedGB / memTotalGB * 100
	}
	ts := time.Now().UnixNano()
	return fmt.Sprintf("server_metrics,agent_id=%s,hostname=%s cpu=%.2f,mem=%.2f,mem_used_gb=%.3f,mem_total_gb=%.3f,disk_used_gb=%.3f,disk_total_gb=%.3f,containers=%d,net_down_mbps=%.3f,net_up_mbps=%.3f %d\n",
		escapeTag(agentID), escapeTag(hostname),
		cpuPercent, memPercent, memUsedGB, memTotalGB, diskUsedGB, diskTotalGB,
		containers, netDownMbps, netUpMbps, ts)
}
