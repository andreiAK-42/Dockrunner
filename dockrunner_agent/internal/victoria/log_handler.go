package victoria

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

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
