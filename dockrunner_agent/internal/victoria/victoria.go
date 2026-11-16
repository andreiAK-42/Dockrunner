// Package victoria — отправка метрик в VictoriaMetrics и логов в VictoriaLogs.
// Оба приёмника принимают данные по HTTP: метрики — в формате Influx line
// protocol (POST /write), логи — JSON lines (POST /insert/jsonline).
// Аутентификация — Bearer-токен (например, через vmauth); пустой токен —
// без заголовка (доверенная сеть).
package victoria

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
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

// logSender — общая асинхронная отправка JSON-lines в VictoriaLogs.
// Очередь ограничена: при переполнении свежие записи дропаются,
// чтобы медленная Victoria не забила память сервиса.
type logSender struct {
	endpoint string
	token    string
	client   *http.Client
	logger   *slog.Logger

	mu     sync.Mutex
	closed bool
	ch     chan map[string]any
	done   chan struct{}
}

func newLogSender(rawURL, token string, timeout time.Duration, logger *slog.Logger) *logSender {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	// vl_stream-fields: поля, по которым VictoriaLogs делает stream (индекс).
	endpoint := strings.TrimRight(rawURL, "/") + "/insert/jsonline?vl_stream-fields=service,env,host"
	s := &logSender{
		endpoint: endpoint,
		token:    token,
		client:   &http.Client{Timeout: timeout},
		logger:   logger,
		ch:       make(chan map[string]any, 4096),
		done:     make(chan struct{}),
	}
	go s.loop()
	return s
}

const (
	logBatchSize    = 256
	logFlushEvery   = 500 * time.Millisecond
	logQueueTimeout = 50 * time.Millisecond
)

func (s *logSender) loop() {
	defer close(s.done)
	batch := make([]map[string]any, 0, logBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		s.deliver(batch)
		batch = make([]map[string]any, 0, logBatchSize)
	}
	for {
		timer := time.NewTimer(logFlushEvery)
		select {
		case rec, ok := <-s.ch:
			if !timer.Stop() {
				<-timer.C
			}
			if !ok {
				flush()
				return
			}
			batch = append(batch, rec)
			if len(batch) >= logBatchSize {
				flush()
			}
		case <-timer.C:
			flush()
		}
	}
}

// deliver шлёт батч; ошибка — только в лог, запись не повторяется.
func (s *logSender) deliver(batch []map[string]any) {
	var buf bytes.Buffer
	for _, rec := range batch {
		raw, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		buf.Write(raw)
		buf.WriteByte('\n')
	}
	req, err := http.NewRequest(http.MethodPost, s.endpoint, &buf)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/stream+json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	res, err := s.client.Do(req)
	if err != nil {
		s.logger.Warn("victoria logs send failed", "error", err)
		return
	}
	_ = res.Body.Close()
	if res.StatusCode >= 300 {
		s.logger.Warn("victoria logs rejected", "status", res.StatusCode)
	}
}

// Send ставит запись в очередь без блокировки вызывающего.
func (s *logSender) Send(rec map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- rec:
	default:
		// Очередь полна — дропаем, чтобы не блокировать сервис.
		select {
		case s.ch <- rec:
		case <-time.After(logQueueTimeout):
		}
	}
}

func (s *logSender) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.ch)
	s.mu.Unlock()
	<-s.done
}

// LogHandler — slog.Handler, дублирующий записи в VictoriaLogs.
type LogHandler struct {
	sender *logSender
	host   string
	attrs  []any // чередующиеся ключ-значение из With()
	groups []string
	level  slog.Level
}

// NewLogHandler собирает handler. service — имя сервиса (stream-поле),
// host — имя узла. Записи уровнем ниже minLevel игнорируются.
func NewLogHandler(senderLogURL, token, service, env, host string, minLevel slog.Level, timeout time.Duration, logger *slog.Logger) *LogHandler {
	sender := newLogSender(senderLogURL, token, timeout, logger)
	h := &LogHandler{sender: sender, host: host, level: minLevel}
	h.attrs = []any{"service", service, "env", env}
	return h
}

// Close завершает отправку: дожидается опустошения очереди.
func (h *LogHandler) Close() { h.sender.Close() }

func (h *LogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]any{}, h.attrs...), attrsToPairs(attrs)...)
	return &clone
}

func (h *LogHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

func attrsToPairs(attrs []slog.Attr) []any {
	pairs := make([]any, 0, len(attrs)*2)
	for _, a := range attrs {
		pairs = append(pairs, a.Key, a.Value.Any())
	}
	return pairs
}

func (h *LogHandler) Handle(_ context.Context, r slog.Record) error {
	rec := map[string]any{
		"time":    r.Time.UTC().Format(time.RFC3339Nano),
		"level":   r.Level.String(),
		"message": r.Message,
		"host":    h.host,
	}
	for i := 0; i+1 < len(h.attrs); i += 2 {
		rec[fmt.Sprint(h.attrs[i])] = h.attrs[i+1]
	}
	r.Attrs(func(a slog.Attr) bool {
		key := a.Key
		if len(h.groups) > 0 {
			key = strings.Join(append(append([]string{}, h.groups...), key), ".")
		}
		rec[key] = a.Value.Any()
		return true
	})
	h.sender.Send(rec)
	return nil
}

// MultiHandler рассылает запись всем handler'ам (stdout + VictoriaLogs).
type MultiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		clone[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: clone}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	clone := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		clone[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: clone}
}

func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}
