// Package vlogs — асинхронная отправка логов в VictoriaLogs.
// Приёмник принимает JSON lines: POST /insert/jsonline?vl_stream-fields=...
// Аутентификация — Bearer-токен (например, через vmauth).
// Очередь ограничена: при переполнении записи дропаются, чтобы медленная
// Victoria не забила память сервиса; ошибки отправки пишутся в stderr.
package vlogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Sender struct {
	endpoint string
	token    string
	client   *http.Client

	mu     sync.Mutex
	closed bool
	ch     chan map[string]any
	done   chan struct{}
}

// New собирает sender. rawURL — базовый адрес, например http://victoria-logs:9428.
// streamFields — поля записи, по которым VictoriaLogs строит stream (индекс).
func New(rawURL, token string, timeout time.Duration, streamFields ...string) *Sender {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	q := ""
	if len(streamFields) > 0 {
		q = "?vl_stream-fields=" + strings.Join(streamFields, ",")
	}
	s := &Sender{
		endpoint: strings.TrimRight(rawURL, "/") + "/insert/jsonline" + q,
		token:    token,
		client:   &http.Client{Timeout: timeout},
		ch:       make(chan map[string]any, 4096),
		done:     make(chan struct{}),
	}
	go s.loop()
	return s
}

const (
	batchSize    = 256
	flushEvery   = 500 * time.Millisecond
	queueTimeout = 50 * time.Millisecond
)

func (s *Sender) loop() {
	defer close(s.done)
	batch := make([]map[string]any, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		s.deliver(batch)
		batch = make([]map[string]any, 0, batchSize)
	}
	for {
		timer := time.NewTimer(flushEvery)
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
			if len(batch) >= batchSize {
				flush()
			}
		case <-timer.C:
			flush()
		}
	}
}

// deliver шлёт батч; ошибка — в stderr, повторов нет (логи недорогие).
func (s *Sender) deliver(batch []map[string]any) {
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
		fmt.Fprintf(os.Stderr, "victoria logs send failed: %v\n", err)
		return
	}
	_ = res.Body.Close()
	if res.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "victoria logs rejected: status %d\n", res.StatusCode)
	}
}

// Send ставит запись в очередь, не блокируя вызывающего.
func (s *Sender) Send(rec map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- rec:
	default:
		select {
		case s.ch <- rec:
		case <-time.After(queueTimeout):
		}
	}
}

// Close завершает отправку, дожидаясь опустошения очереди.
func (s *Sender) Close() {
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
