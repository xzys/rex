// Package keyrepeat provides canonical auto-repeat constants and a reusable
// KeyRepeater that emits synthetic "key held" events with a configurable
// initial delay and steady repeat interval.
//
// Gamepad/joystick input does not auto-repeat at the hardware/SDL level, so
// "hold to repeat" must be manufactured in software. The constants here are
// the single source of truth shared by server-side operators and keyboard
// client models.
package keyrepeat

import (
	"sync"
	"time"
)

const (
	// Delay is the time a key must be held before the first auto-repeat fires.
	Delay = 300 * time.Millisecond

	// Interval is the steady cadence between subsequent auto-repeats.
	Interval = 70 * time.Millisecond

	// PTTInterval is the push-to-talk Space-spam interval used by Claude
	// voice mode — continuous hold, no initial delay. It is not typematic
	// repeat; it's a distinct constant for the distinct use case.
	PTTInterval = 30 * time.Millisecond
)

// KeyRepeater fires synthetic "key held" callbacks at the configured
// delay/interval. Start begins repeating; Stop stops. Start is idempotent for
// the same key and switches to a new key (stopping the prior one). Safe for
// concurrent use.
type KeyRepeater struct {
	handler  func(key int) error
	delay    time.Duration
	interval time.Duration

	mu   sync.Mutex
	key  int
	stop chan struct{}
}

// NewKeyRepeater creates a KeyRepeater. handler is called for each synthetic
// repeat with the held key. delay is the wait before the first repeat (0 =
// fire immediately). interval is the steady repeat cadence.
func NewKeyRepeater(handler func(key int) error, delay, interval time.Duration) *KeyRepeater {
	return &KeyRepeater{handler: handler, delay: delay, interval: interval, key: -1}
}

// Start begins repeating for the given key. If already repeating the same key
// the call is a no-op. Starting a different key stops the prior one first.
func (r *KeyRepeater) Start(key int) {
	r.mu.Lock()
	if r.key == key && r.stop != nil {
		r.mu.Unlock()
		return
	}
	r.stopLocked()
	stop := make(chan struct{})
	r.key = key
	r.stop = stop
	r.mu.Unlock()

	go r.loop(key, stop)
}

func (r *KeyRepeater) loop(key int, stop <-chan struct{}) {
	// Initial delay phase.
	if r.delay > 0 {
		timer := time.NewTimer(r.delay)
		select {
		case <-stop:
			timer.Stop()
			return
		case <-timer.C:
		}
	}

	// First repeat.
	if err := r.handler(key); err != nil {
		return
	}

	// Steady repeat.
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := r.handler(key); err != nil {
				return
			}
		}
	}
}

// Stop stops the repeater for the given key. If a different key is active the
// call is a no-op.
func (r *KeyRepeater) Stop(key int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.key != key {
		return
	}
	r.stopLocked()
}

// StopAll stops the repeater regardless of which key is active.
func (r *KeyRepeater) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

func (r *KeyRepeater) stopLocked() {
	if r.stop != nil {
		close(r.stop)
		r.stop = nil
	}
	r.key = -1
}
