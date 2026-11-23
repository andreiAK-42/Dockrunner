package victoria

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

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
