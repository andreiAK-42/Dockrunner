package buildlog

import (
	"database/sql"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxLines = 10000
const keepFor = 24 * time.Hour

// Store keeps build log lines per deployment id in SQLite,
// so logs survive agent restarts.
type Store struct {
	mu   sync.Mutex
	db   *sql.DB
	last map[string]string
	subs map[string]map[chan string]struct{}
}

func New(db *sql.DB) *Store {
	s := &Store{db: db, last: map[string]string{}, subs: map[string]map[chan string]struct{}{}}
	if db != nil {
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS build_logs(
			deployment_id TEXT NOT NULL, line TEXT NOT NULL, created_at INTEGER NOT NULL)`)
		_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_build_logs_dep ON build_logs(deployment_id)`)
		_, _ = db.Exec(`DELETE FROM build_logs WHERE created_at < ?`, time.Now().Add(-keepFor).Unix())
	}
	return s
}

// Append adds a line, skipping empty lines and consecutive duplicates
// (docker progress updates spam the same status dozens of times).
func (s *Store) Append(id, line string) {
	line = strings.TrimRight(line, "\n")
	if line == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last[id] == line {
		return
	}
	s.last[id] = line
	if s.db == nil {
		return
	}
	_, _ = s.db.Exec(`INSERT INTO build_logs(deployment_id, line, created_at) VALUES (?, ?, ?)`,
		id, line, time.Now().Unix())
	_, _ = s.db.Exec(`DELETE FROM build_logs WHERE deployment_id = ? AND rowid NOT IN
		(SELECT rowid FROM build_logs WHERE deployment_id = ? ORDER BY rowid DESC LIMIT ?)`, id, id, maxLines)
	for ch := range s.subs[id] {
		select {
		case ch <- line:
		default:
		}
	}
}

// Subscribe returns a channel receiving new lines for id until unsubscribed.
func (s *Store) Subscribe(id string) (<-chan string, func()) {
	ch := make(chan string, 64)
	s.mu.Lock()
	if s.subs[id] == nil {
		s.subs[id] = map[chan string]struct{}{}
	}
	s.subs[id][ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if set, ok := s.subs[id]; ok {
			delete(set, ch)
			close(ch)
		}
	}
}

// Tail returns the last n lines joined, or false when nothing is stored.
func (s *Store) Tail(id string, n int) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return "", false
	}
	if n <= 0 || n > maxLines {
		n = maxLines
	}
	rows, err := s.db.Query(`SELECT line FROM build_logs WHERE deployment_id = ?
		ORDER BY rowid DESC LIMIT ?`, id, n)
	if err != nil {
		return "", false
	}
	defer func() { _ = rows.Close() }()
	lines := []string{}
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", false
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "", false
	}
	slices.Reverse(lines)
	return strings.Join(lines, "\n"), true
}
