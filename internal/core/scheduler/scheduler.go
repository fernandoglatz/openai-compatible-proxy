package scheduler

import (
	"context"
	"sync"
	"time"
)

// Scheduler serializes generation requests so the backend keeps one conversation's
// context warm at a time. Capacity is one in-flight request. The session that most
// recently held the slot ("active") keeps priority: its next request jumps ahead of
// waiting sessions. A different session takes over only when the active session's
// last turn is done, or after an idle grace window elapses.
type Scheduler struct {
	idleTimeout time.Duration

	mu       sync.Mutex
	active   string    // session key currently entitled to the slot ("" = none)
	busy     bool      // a request is currently in flight
	queue    []*waiter // FIFO of waiting requests
	timer    *time.Timer
	timerGen uint64 // invalidates stale idle-timer callbacks
}

type waiter struct {
	key   string
	ready chan struct{} // closed when the slot is granted to this waiter
}

func NewScheduler(idleTimeout time.Duration) *Scheduler {
	return &Scheduler{idleTimeout: idleTimeout}
}

// Acquire blocks until this request may run, then returns a release function that
// MUST be called exactly once when the request finishes. done reports whether the
// session's turn is complete (true -> a waiting session may take over immediately;
// false -> hold the slot for the grace window so the active session can reclaim it).
// If ctx is canceled while waiting, Acquire returns (nil, ctx.Err()).
func (s *Scheduler) Acquire(ctx context.Context, key string) (func(done bool), error) {
	s.mu.Lock()
	if !s.busy && (s.active == "" || s.active == key) {
		s.grantLocked(key)
		s.mu.Unlock()
		return s.releaseOnce(), nil
	}

	w := &waiter{key: key, ready: make(chan struct{})}
	s.queue = append(s.queue, w)
	s.mu.Unlock()

	select {
	case <-w.ready:
		return s.releaseOnce(), nil
	case <-ctx.Done():
		s.mu.Lock()
		select {
		case <-w.ready:
			// Granted concurrently with cancellation: release so the slot is not
			// leaked, treating the vanished session as done for a prompt handoff.
			s.mu.Unlock()
			s.releaseOnce()(true)
		default:
			s.removeWaiterLocked(w)
			s.mu.Unlock()
		}
		return nil, ctx.Err()
	}
}

func (s *Scheduler) releaseOnce() func(bool) {
	var once sync.Once
	return func(done bool) {
		once.Do(func() { s.release(done) })
	}
}

func (s *Scheduler) grantLocked(key string) {
	s.active = key
	s.busy = true
	s.cancelTimerLocked()
}

func (s *Scheduler) release(done bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = false

	// 1. Same-session request already waiting -> keep it warm, grant immediately.
	if w := s.popWaiterLocked(s.active); w != nil {
		s.busy = true
		close(w.ready)
		return
	}
	// 2. Nothing else waiting -> release ownership; next arrival starts fresh.
	if len(s.queue) == 0 {
		s.active = ""
		return
	}
	// 3. Other sessions waiting and the active session is done -> switch now.
	if done {
		s.promoteHeadLocked()
		return
	}
	// 4. Not done -> hold the slot for the grace window; if the active session
	// does not reclaim it, the timer promotes the head of the queue.
	s.armIdleTimerLocked()
}

func (s *Scheduler) popWaiterLocked(key string) *waiter {
	if key == "" {
		return nil
	}
	for i, w := range s.queue {
		if w.key == key {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			return w
		}
	}
	return nil
}

func (s *Scheduler) removeWaiterLocked(target *waiter) {
	for i, w := range s.queue {
		if w == target {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			return
		}
	}
}

func (s *Scheduler) promoteHeadLocked() {
	w := s.queue[0]
	s.queue = s.queue[1:]
	s.active = w.key
	s.busy = true
	close(w.ready)
}

func (s *Scheduler) cancelTimerLocked() {
	s.timerGen++
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *Scheduler) armIdleTimerLocked() {
	s.timerGen++
	gen := s.timerGen
	s.timer = time.AfterFunc(s.idleTimeout, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if gen != s.timerGen {
			return // superseded by a newer arm/cancel
		}
		if !s.busy && len(s.queue) > 0 {
			s.promoteHeadLocked()
		}
	})
}
