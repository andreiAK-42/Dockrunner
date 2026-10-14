package logs

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"log/slog"

	"dockrunner_agent/internal/docker"
)

const bufferSize = 500

type Entry struct {
	Stream string `json:"stream"`
	Line   string `json:"line"`
	Time   string `json:"time"`
}

type Streamer struct {
	dock   *docker.Client
	logger *slog.Logger

	mu      sync.Mutex
	buffers map[string][]Entry
	subs    map[string]map[chan string]struct{}
	cancels map[string]context.CancelFunc
}

func New(dock *docker.Client, logger *slog.Logger) *Streamer {
	return &Streamer{
		dock:    dock,
		logger:  logger,
		buffers: map[string][]Entry{},
		subs:    map[string]map[chan string]struct{}{},
		cancels: map[string]context.CancelFunc{},
	}
}

func (s *Streamer) dispatch(containerID, stream, line string) {
	entry := Entry{Stream: stream, Line: line, Time: time.Now().UTC().Format(time.RFC3339)}
	raw, _ := json.Marshal(entry)
	text := string(raw)

	s.mu.Lock()
	defer s.mu.Unlock()
	buf := append(s.buffers[containerID], entry)
	if len(buf) > bufferSize {
		buf = append([]Entry{}, buf[len(buf)-bufferSize:]...)
	}
	s.buffers[containerID] = buf
	for ch := range s.subs[containerID] {
		select {
		case ch <- text:
		default:
		}
	}
}

func (s *Streamer) Attach(containerID, containerName string) {
	s.mu.Lock()
	if _, ok := s.cancels[containerID]; ok {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancels[containerID] = cancel
	s.mu.Unlock()

	go func() {
		err := s.dock.FollowLogs(ctx, containerID, func(stream, line string) {
			s.dispatch(containerID, stream, line)
		})
		if err != nil {
			s.logger.Warn("log follow stopped", "container", containerName, "error", err)
		}
		s.mu.Lock()
		delete(s.cancels, containerID)
		s.mu.Unlock()
	}()
}

func (s *Streamer) Detach(containerID string) {
	s.mu.Lock()
	cancel, ok := s.cancels[containerID]
	s.mu.Unlock()
	if ok {
		cancel()
	}
}

func (s *Streamer) History(containerID string, n int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf := s.buffers[containerID]
	if n > 0 && len(buf) > n {
		buf = buf[len(buf)-n:]
	}
	out := make([]string, 0, len(buf))
	for _, e := range buf {
		raw, _ := json.Marshal(e)
		out = append(out, string(raw))
	}
	return out
}

func (s *Streamer) Subscribe(containerID string) (<-chan string, func()) {
	ch := make(chan string, 64)
	s.mu.Lock()
	if s.subs[containerID] == nil {
		s.subs[containerID] = map[chan string]struct{}{}
	}
	s.subs[containerID][ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if set, ok := s.subs[containerID]; ok {
			delete(set, ch)
			close(ch)
		}
	}
}

func (s *Streamer) Stop() {
	s.mu.Lock()
	cancels := s.cancels
	s.cancels = map[string]context.CancelFunc{}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
