package scheduler

import (
	"context"
	"testing"
	"time"
)

func mustAcquire(t *testing.T, s *Scheduler, key string) func(bool) {
	t.Helper()
	rel, err := s.Acquire(context.Background(), key)
	if err != nil {
		t.Fatalf("Acquire(%s) unexpected error: %v", key, err)
	}
	return rel
}

// granted returns a channel closed when Acquire(key) returns. The release is
// invoked with the given done value after grant.
func granted(s *Scheduler, key string, done bool) chan struct{} {
	ch := make(chan struct{})
	go func() {
		rel, err := s.Acquire(context.Background(), key)
		if err != nil {
			return
		}
		close(ch)
		rel(done)
	}()
	return ch
}

func assertClosedWithin(t *testing.T, ch chan struct{}, d time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(d):
		t.Fatal(msg)
	}
}

func assertNotClosedWithin(t *testing.T, ch chan struct{}, d time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(msg)
	case <-time.After(d):
	}
}

func TestSingleSessionSequential(t *testing.T) {
	s := NewScheduler(50 * time.Millisecond)
	rel := mustAcquire(t, s, "A")
	rel(true)
	// Same key, no contention: must be granted immediately.
	assertClosedWithin(t, granted(s, "A", true), 100*time.Millisecond, "second A request blocked")
}

func TestDifferentSessionWaitsThenGranted(t *testing.T) {
	s := NewScheduler(50 * time.Millisecond)
	relA := mustAcquire(t, s, "A")
	b := granted(s, "B", true)
	assertNotClosedWithin(t, b, 20*time.Millisecond, "B granted while A held the slot")
	relA(true) // done -> immediate switch
	assertClosedWithin(t, b, 200*time.Millisecond, "B not granted after A finished")
}

func TestActiveSessionPreemptsWaiter(t *testing.T) {
	s := NewScheduler(50 * time.Millisecond)
	relA1 := mustAcquire(t, s, "A") // A active, busy
	b := granted(s, "B", true)      // B enqueues
	time.Sleep(10 * time.Millisecond)
	a2 := granted(s, "A", false) // A2 enqueues behind B but same session
	time.Sleep(10 * time.Millisecond)
	relA1(false) // A1 ends with tool_calls; A2 (same session) must jump ahead of B
	assertClosedWithin(t, a2, 200*time.Millisecond, "A2 not granted before B")
	assertNotClosedWithin(t, b, 20*time.Millisecond, "B granted before A2 completed")
}

func TestIdleSwitchAfterTimeout(t *testing.T) {
	s := NewScheduler(40 * time.Millisecond)
	relA := mustAcquire(t, s, "A")
	b := granted(s, "B", true)
	time.Sleep(10 * time.Millisecond)
	relA(false) // tool_calls + B waiting -> arm idle timer, no immediate switch
	assertNotClosedWithin(t, b, 20*time.Millisecond, "B switched before idle timeout")
	assertClosedWithin(t, b, 300*time.Millisecond, "B not granted after idle timeout")
}

func TestCancelWhileWaiting(t *testing.T) {
	s := NewScheduler(50 * time.Millisecond)
	relA := mustAcquire(t, s, "A")

	cctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := s.Acquire(cctx, "B")
		errCh <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("canceled Acquire returned nil error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("canceled Acquire did not return")
	}

	relA(true)
	// The canceled B must have left the queue, so C acquires promptly.
	assertClosedWithin(t, granted(s, "C", true), 200*time.Millisecond, "C blocked after B canceled")
}
