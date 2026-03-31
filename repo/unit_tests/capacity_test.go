package unit_tests

import (
	"errors"
	"sync"
	"testing"
	"time"
)

var (
	errConflict = errors.New("capacity conflict")
	errExpired  = errors.New("hold expired")
)

type hold struct {
	id         string
	stallCount int
	expiresAt  time.Time
	confirmed  bool
	cancelled  bool
}

type capacityEngine struct {
	mu         sync.Mutex
	total      int
	nextID     int
	holds      map[string]hold
	nowFunc    func() time.Time
	holdWindow time.Duration
}

func newCapacityEngine(total int, timeout time.Duration) *capacityEngine {
	return &capacityEngine{
		total:      total,
		nextID:     1,
		holds:      map[string]hold{},
		nowFunc:    func() time.Time { return time.Now().UTC() },
		holdWindow: timeout,
	}
}

func (e *capacityEngine) now() time.Time { return e.nowFunc() }

func (e *capacityEngine) Hold(stalls int) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.expireLocked()
	if e.availableLocked() < stalls {
		return "", errConflict
	}
	id := "h"
	if e.nextID == 1 {
		id = "h1"
	} else {
		id = "h" + time.Unix(int64(e.nextID), 0).UTC().Format("150405")
	}
	e.nextID++
	e.holds[id] = hold{id: id, stallCount: stalls, expiresAt: e.now().Add(e.holdWindow)}
	return id, nil
}

func (e *capacityEngine) Confirm(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.expireLocked()
	h, ok := e.holds[id]
	if !ok || h.cancelled {
		return errExpired
	}
	if h.confirmed {
		return nil
	}
	if h.expiresAt.Before(e.now()) || h.expiresAt.Equal(e.now()) {
		delete(e.holds, id)
		return errExpired
	}
	h.confirmed = true
	e.holds[id] = h
	return nil
}

func (e *capacityEngine) Cancel(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h, ok := e.holds[id]
	if !ok {
		return
	}
	h.cancelled = true
	delete(e.holds, id)
}

func (e *capacityEngine) availableLocked() int {
	used := 0
	for _, h := range e.holds {
		if h.cancelled {
			continue
		}
		used += h.stallCount
	}
	return e.total - used
}

func (e *capacityEngine) Available() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.expireLocked()
	return e.availableLocked()
}

func (e *capacityEngine) expireLocked() {
	now := e.now()
	for id, h := range e.holds {
		if !h.confirmed && (h.expiresAt.Before(now) || h.expiresAt.Equal(now)) {
			delete(e.holds, id)
		}
	}
}

func TestHoldAtomicityConcurrentLastStall(t *testing.T) {
	engine := newCapacityEngine(1, 15*time.Minute)
	engine.nowFunc = func() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := engine.Hold(1)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, errConflict) {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected 1 success and 1 conflict, got success=%d conflict=%d", successes, conflicts)
	}
}

func TestHoldExpiryRestoresCapacity(t *testing.T) {
	engine := newCapacityEngine(2, time.Minute)
	current := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	engine.nowFunc = func() time.Time { return current }

	_, err := engine.Hold(2)
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	if got := engine.Available(); got != 0 {
		t.Fatalf("expected no available stalls after hold, got %d", got)
	}

	current = current.Add(2 * time.Minute)
	if got := engine.Available(); got != 2 {
		t.Fatalf("expected stalls restored after expiry, got %d", got)
	}
}

func TestConfirmRecheckFailsAfterExpiry(t *testing.T) {
	engine := newCapacityEngine(1, time.Minute)
	current := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	engine.nowFunc = func() time.Time { return current }

	id, err := engine.Hold(1)
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	current = current.Add(2 * time.Minute)
	if err := engine.Confirm(id); !errors.Is(err, errExpired) {
		t.Fatalf("expected expired error on confirm, got %v", err)
	}
}

func TestCancelReleasesStalls(t *testing.T) {
	engine := newCapacityEngine(3, 10*time.Minute)
	engine.nowFunc = func() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }
	id, err := engine.Hold(2)
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	if got := engine.Available(); got != 1 {
		t.Fatalf("expected 1 stall available, got %d", got)
	}
	engine.Cancel(id)
	if got := engine.Available(); got != 3 {
		t.Fatalf("expected all stalls available after cancel, got %d", got)
	}
}
