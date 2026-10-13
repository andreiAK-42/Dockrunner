package oplock

import (
	"errors"
	"fmt"
	"sync"
)

// ErrBusy is returned when another operation already runs for the same app.
var ErrBusy = errors.New("busy")

// New returns an empty Tracker.
// Tracker blocks concurrent operations on the same application (owner/app).
// Long operations (check/deploy/refresh) acquire the key and release it
// when a terminal event arrives; a second attempt gets ErrBusy -> HTTP 409.
// Zero value is ready to use.
func New() *Tracker {
	return &Tracker{busy: map[string]string{}}
}

type Tracker struct {
	mu   sync.Mutex
	busy map[string]string
}

func (t *Tracker) key(owner, app string) string {
	return owner + "/" + app
}

// Acquire locks owner/app for op. Returns an error wrapping ErrBusy naming
// the running operation when the app is already locked.
func (t *Tracker) Acquire(owner, app, op string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.busy == nil {
		t.busy = map[string]string{}
	}
	if running, ok := t.busy[t.key(owner, app)]; ok {
		return fmt.Errorf("%w: сейчас выполняется операция %q — дождитесь её завершения", ErrBusy, running)
	}
	t.busy[t.key(owner, app)] = op
	return nil
}

// Release unlocks owner/app. Safe to call for a non-locked app.
func (t *Tracker) Release(owner, app string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.busy, t.key(owner, app))
}

// Holder returns the running operation name, or "" when the app is free.
func (t *Tracker) Holder(owner, app string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.busy[t.key(owner, app)]
}
