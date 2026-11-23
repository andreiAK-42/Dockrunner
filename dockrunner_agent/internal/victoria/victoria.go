// Package victoria — отправка метрик в VictoriaMetrics и логов в VictoriaLogs.
// Оба приёмника принимают данные по HTTP: метрики — в формате Influx line
// protocol (POST /write), логи — JSON lines (POST /insert/jsonline).
// Аутентификация — Bearer-токен (например, через vmauth); пустой токен —
// без заголовка (доверенная сеть).
package victoria

import (
	"context"
	"log/slog"
)

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
