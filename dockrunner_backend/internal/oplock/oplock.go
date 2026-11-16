package oplock

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrBusy is returned when another operation already runs for the same app.
var ErrBusy = errors.New("busy")

// DefaultTTL — сколько замок держится без терминального события.
// Событие из RabbitMQ может потеряться (брокер рестартовал, publish без confirm) —
// без TTL приложение осталось бы заблокированным навсегда.
const DefaultTTL = 15 * time.Minute

// New returns an empty Tracker.
// Tracker blocks concurrent operations on the same application (owner/app).
// Long operations (check/deploy/refresh) acquire the key and release it
// when a terminal event arrives; a second attempt gets ErrBusy -> HTTP 409.
// Zero value is ready to use.
func New() *Tracker {
	return &Tracker{}
}

// NewWithTTL returns a Tracker with a custom lock TTL (<=0 — DefaultTTL).
func NewWithTTL(ttl time.Duration) *Tracker {
	return &Tracker{ttl: ttl}
}

type opEntry struct {
	op    string
	since time.Time
}

type Tracker struct {
	mu   sync.Mutex
	busy map[string]opEntry
	ttl  time.Duration
}

func (t *Tracker) key(owner, app string) string {
	return owner + "/" + app
}

func (t *Tracker) ttlOrDefault() time.Duration {
	if t.ttl > 0 {
		return t.ttl
	}
	return DefaultTTL
}

// expireLocked снимает протухшие замки. Вызывается под mu.
func (t *Tracker) expireLocked(now time.Time) {
	ttl := t.ttlOrDefault()
	for k, e := range t.busy {
		if now.Sub(e.since) > ttl {
			delete(t.busy, k)
		}
	}
}

// Acquire locks owner/app for op. Returns an error wrapping ErrBusy naming
// the running operation when the app is already locked. A lock older than
// the TTL is treated as lost (its terminal event never arrived) and re-taken.
func (t *Tracker) Acquire(owner, app, op string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.busy == nil {
		t.busy = map[string]opEntry{}
	}
	t.expireLocked(time.Now())
	if running, ok := t.busy[t.key(owner, app)]; ok {
		return fmt.Errorf("%w: сейчас выполняется операция %q — дождитесь её завершения", ErrBusy, running.op)
	}
	t.busy[t.key(owner, app)] = opEntry{op: op, since: time.Now()}
	return nil
}

// Release unlocks owner/app. Safe to call for a non-locked app.
func (t *Tracker) Release(owner, app string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.busy, t.key(owner, app))
}

// Holder returns the running operation name, or "" when the app is free.
// An expired lock reports as free.
func (t *Tracker) Holder(owner, app string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.expireLocked(time.Now())
	return t.busy[t.key(owner, app)].op
}
