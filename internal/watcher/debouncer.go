package watcher

import (
	"sync"
	"time"
)

// Debouncer coalesces rapid events for the same key within a configurable
// window. Only the last event for each key fires after the window elapses.
type Debouncer struct {
	mu      sync.Mutex
	window  time.Duration
	timers  map[string]*time.Timer
}

// NewDebouncer creates a Debouncer with the given coalescing window.
func NewDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{
		window: window,
		timers: make(map[string]*time.Timer),
	}
}

// Debounce schedules fn to run after the debounce window for the given key.
// If Debounce is called again for the same key before the window elapses, the
// previous timer is reset and only the latest fn will fire.
func (d *Debouncer) Debounce(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if t, ok := d.timers[key]; ok {
		t.Stop()
	}

	d.timers[key] = time.AfterFunc(d.window, func() {
		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
		fn()
	})
}

// Stop cancels all pending timers.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, t := range d.timers {
		t.Stop()
		delete(d.timers, key)
	}
}
