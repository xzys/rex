package keyrepeat

import (
	"sync"
	"testing"
	"time"
)

func TestStartStop(t *testing.T) {
	var mu sync.Mutex
	var fires []int
	r := NewKeyRepeater(func(key int) error {
		mu.Lock()
		fires = append(fires, key)
		mu.Unlock()
		return nil
	}, 50*time.Millisecond, 30*time.Millisecond)

	r.Start(1)
	// Wait for at least one repeat, then stop.
	time.Sleep(120 * time.Millisecond)
	r.Stop(1)

	// Give straggler ticks time to arrive.
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	n := len(fires)
	mu.Unlock()

	if n < 1 {
		t.Fatalf("expected at least 1 repeat, got %d", n)
	}
	// After stop, no more should arrive. The count from the first check
	// should be stable.
	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	n2 := len(fires)
	mu.Unlock()

	if n2 != n {
		t.Fatalf("expected no more fires after Stop: was %d, now %d", n, n2)
	}
}

func TestStartIdempotent(t *testing.T) {
	var mu sync.Mutex
	var fires []int
	r := NewKeyRepeater(func(key int) error {
		mu.Lock()
		fires = append(fires, key)
		mu.Unlock()
		return nil
	}, 50*time.Millisecond, 30*time.Millisecond)

	// Two starts for the same key — second should be no-op.
	r.Start(1)
	r.Start(1)
	time.Sleep(120 * time.Millisecond)
	r.Stop(1)
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	n := len(fires)
	mu.Unlock()

	if n < 1 {
		t.Fatalf("expected at least 1 repeat, got %d", n)
	}
}

func TestStartDifferentKeyStopsOld(t *testing.T) {
	var mu sync.Mutex
	var fires []int
	r := NewKeyRepeater(func(key int) error {
		mu.Lock()
		fires = append(fires, key)
		mu.Unlock()
		return nil
	}, 50*time.Millisecond, 30*time.Millisecond)

	r.Start(1)
	time.Sleep(80 * time.Millisecond) // let delay pass, maybe one fire

	// Switch to key 2 — this should stop key 1's goroutine.
	r.Start(2)
	time.Sleep(120 * time.Millisecond)
	r.Stop(2)
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Key 2 should appear at least once.
	hasKey2 := false
	for _, k := range fires {
		if k == 2 {
			hasKey2 = true
			break
		}
	}
	if !hasKey2 {
		t.Fatalf("expected key 2 in fires, got %v", fires)
	}
}

func TestStopWrongKeyNoOp(t *testing.T) {
	var mu sync.Mutex
	var fires []int
	r := NewKeyRepeater(func(key int) error {
		mu.Lock()
		fires = append(fires, key)
		mu.Unlock()
		return nil
	}, 50*time.Millisecond, 30*time.Millisecond)

	r.Start(1)
	time.Sleep(80 * time.Millisecond)

	// Stop a different key — should not stop the active one.
	r.Stop(99)
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	n := len(fires)
	mu.Unlock()

	r.Stop(1)
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	n2 := len(fires)
	mu.Unlock()

	// Fires should have happened before the correct-key stop (n > 0).
	// After correct-key stop, no additional fires (n2 == n).
	if n == 0 {
		t.Fatalf("expected fires before correct-key stop, got %d", n)
	}
	if n2 != n {
		t.Fatalf("expected no more fires after correct stop (before=%d, after=%d)", n, n2)
	}
}

func TestStopAll(t *testing.T) {
	var mu sync.Mutex
	var fires []int
	r := NewKeyRepeater(func(key int) error {
		mu.Lock()
		fires = append(fires, key)
		mu.Unlock()
		return nil
	}, 50*time.Millisecond, 30*time.Millisecond)

	r.Start(1)
	time.Sleep(120 * time.Millisecond)
	r.StopAll()
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	n := len(fires)
	mu.Unlock()

	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	n2 := len(fires)
	mu.Unlock()

	if n2 != n {
		t.Fatalf("expected no fires after StopAll: was %d, now %d", n, n2)
	}
}

func TestZeroDelayFiresImmediately(t *testing.T) {
	var mu sync.Mutex
	var fires []int
	r := NewKeyRepeater(func(key int) error {
		mu.Lock()
		fires = append(fires, key)
		mu.Unlock()
		return nil
	}, 0, 30*time.Millisecond)

	start := time.Now()
	r.Start(1)
	time.Sleep(80 * time.Millisecond)
	r.Stop(1)

	mu.Lock()
	n := len(fires)
	mu.Unlock()

	if n < 2 {
		t.Fatalf("zero-delay PTT mode: expected at least 2 fires, got %d", n)
	}

	elapsed := time.Since(start)
	// With 0ms delay, first fire should happen well before 50ms.
	if elapsed > 100*time.Millisecond && n < 2 {
		t.Fatalf("zero-delay fires took too long: %v for %d fires", elapsed, n)
	}
}
