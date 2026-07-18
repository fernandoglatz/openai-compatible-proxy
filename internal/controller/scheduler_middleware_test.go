package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fernandoglatz/openai-compatible-proxy/internal/core/scheduler"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

func TestSessionKeyOf(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Session-Id", "ses_abc")
	if got := sessionKeyOf(ctx); got != "sid:ses_abc" {
		t.Errorf("sessionKeyOf(header) = %q, want sid:ses_abc", got)
	}

	ctx2, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx2.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	got := sessionKeyOf(ctx2)
	if got == "" || got[:3] != "ip:" {
		t.Errorf("sessionKeyOf(no header) = %q, want ip: prefix", got)
	}
}

func TestIsGatedPath(t *testing.T) {
	gated := []string{"/v1/chat/completions", "/v1/completions", "/v1/responses"}
	yes := []string{
		"/v1/chat/completions",
		"/api/v1/chat/completions",
		"/v1/completions",
		"/api/v1/responses",
	}
	no := []string{"/v1/models", "/v1/embeddings", "/health", "/api/v0/models"}
	for _, p := range yes {
		if !isGatedPath(p, gated) {
			t.Errorf("isGatedPath(%q) = false, want true", p)
		}
	}
	for _, p := range no {
		if isGatedPath(p, gated) {
			t.Errorf("isGatedPath(%q) = true, want false", p)
		}
	}
}

func TestSchedulerMiddlewareDisabledPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sched := scheduler.NewScheduler(10 * time.Millisecond)
	cfg := config.SchedulerConfig{Enabled: false, GatedPaths: []string{"/v1/chat/completions"}}
	engine.Use(SchedulerMiddleware(sched, cfg))
	engine.POST("/v1/chat/completions", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("passthrough failed: code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestSchedulerMiddlewareSerializesGatedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sched := scheduler.NewScheduler(10 * time.Millisecond)
	cfg := config.SchedulerConfig{Enabled: true, GatedPaths: []string{"/v1/chat/completions"}}
	engine.Use(SchedulerMiddleware(sched, cfg))

	var inFlight int32
	var maxSeen int32
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&maxSeen)
			if n <= old || atomic.CompareAndSwapInt32(&maxSeen, old, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		// Terminal finish_reason so the slot is released without waiting the grace window.
		c.String(http.StatusOK, `{"choices":[{"finish_reason":"stop"}]}`)
	})

	var wg sync.WaitGroup
	for i, sid := range []string{"ses_1", "ses_2", "ses_3"} {
		wg.Add(1)
		go func(i int, sid string) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			req.Header.Set("X-Session-Id", sid)
			engine.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("request %d code = %d", i, rec.Code)
			}
		}(i, sid)
	}
	wg.Wait()

	if maxSeen != 1 {
		t.Errorf("max concurrent in-flight = %d, want 1", maxSeen)
	}
}

// TestSchedulerMiddlewareReleasesOnPanic guards against a regression where release()
// was called only after ginCtx.Next() returned normally. If a downstream handler
// panics, gin's Recovery middleware (registered outside the scheduler middleware, as
// in production) unwinds the stack past that call site, so the slot is never released
// and every subsequent gated request deadlocks in Acquire. release() must be deferred
// so it still runs during a panic-driven unwind.
func TestSchedulerMiddlewareReleasesOnPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	sched := scheduler.NewScheduler(10 * time.Millisecond)
	cfg := config.SchedulerConfig{Enabled: true, GatedPaths: []string{"/v1/chat/completions"}}
	engine.Use(SchedulerMiddleware(sched, cfg))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		if c.Request.Header.Get("X-Session-Id") == "panic" {
			panic("boom")
		}
		c.String(http.StatusOK, `{"choices":[{"finish_reason":"stop"}]}`)
	})

	// Request 1: downstream handler panics. Recovery middleware should catch it and
	// respond 500, without hanging.
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req1.Header.Set("X-Session-Id", "panic")
	engine.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf("panic request code = %d, want %d", rec1.Code, http.StatusInternalServerError)
	}

	// Request 2: if the slot leaked from request 1, this blocks forever in
	// sched.Acquire. Bound the wait so the test fails cleanly instead of hanging.
	done := make(chan int, 1)
	go func() {
		rec2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req2.Header.Set("X-Session-Id", "ok")
		engine.ServeHTTP(rec2, req2)
		done <- rec2.Code
	}()

	select {
	case code := <-done:
		if code != http.StatusOK {
			t.Errorf("follow-up request code = %d, want %d", code, http.StatusOK)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follow-up request timed out: scheduler slot leaked after downstream panic")
	}
}

// TestSchedulerMiddlewareRemovesQueuedRequestOnDisconnect drives the full HTTP
// path: a queued request whose client connection closes must be removed from the
// scheduler queue, so the slot it was waiting for is never handed to the departed
// client. This exercises the real net/http connection-close -> request-context
// cancellation -> Acquire ctx.Done -> removeWaiterLocked chain that the unit-level
// TestCancelWhileWaiting only approximates with a manually canceled context.
func TestSchedulerMiddlewareRemovesQueuedRequestOnDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sched := scheduler.NewScheduler(10 * time.Millisecond)
	cfg := config.SchedulerConfig{Enabled: true, GatedPaths: []string{"/v1/chat/completions"}}
	engine.Use(SchedulerMiddleware(sched, cfg))

	started := make(chan string, 8)   // session keys that reached the handler (hold the slot)
	release := make(chan struct{}, 8) // one token lets one held handler finish
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		started <- c.Request.Header.Get("X-Session-Id")
		<-release
		c.String(http.StatusOK, `{"choices":[{"finish_reason":"stop"}]}`)
	})

	srv := httptest.NewServer(engine)
	defer srv.Close()

	// Distinct connection per request so canceling one client closes its own socket.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	post := func(ctx context.Context, sid string) (*http.Response, error) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/chat/completions", nil)
		req.Header.Set("X-Session-Id", sid)
		return client.Do(req)
	}
	drain := func(resp *http.Response) {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// Session A occupies the slot and holds it until released.
	aDone := make(chan struct{})
	go func() {
		if resp, err := post(context.Background(), "A"); err == nil {
			drain(resp)
		}
		close(aDone)
	}()
	select {
	case k := <-started:
		if k != "A" {
			t.Fatalf("expected A to hold the slot first, got %q", k)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("A never reached the handler")
	}

	// Session B arrives and must queue behind A, then disconnects.
	bCtx, bCancel := context.WithCancel(context.Background())
	bErr := make(chan error, 1)
	go func() {
		_, err := post(bCtx, "B")
		bErr <- err
	}()
	time.Sleep(200 * time.Millisecond) // let B reach the server and enqueue
	select {
	case k := <-started:
		t.Fatalf("queued session %q reached the handler while A held the slot", k)
	default: // expected: B is parked in the queue, handler not entered
	}
	bCancel() // client B closes the connection while queued
	select {
	case err := <-bErr:
		if err == nil {
			t.Fatal("B request returned nil error after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("B request did not return after cancellation")
	}

	// Release A. If B was NOT removed from the queue, the freed slot leaks to the
	// departed B and C can never acquire it.
	release <- struct{}{}
	<-aDone

	cResult := make(chan error, 1)
	go func() {
		resp, err := post(context.Background(), "C")
		if err != nil {
			cResult <- err
			return
		}
		drain(resp)
		cResult <- nil
	}()
	select {
	case k := <-started:
		if k != "C" {
			t.Fatalf("expected C to acquire the freed slot, got %q", k)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("C never acquired the slot: queued B was not removed on disconnect (slot leaked)")
	}
	release <- struct{}{}
	if err := <-cResult; err != nil {
		t.Fatalf("C request failed: %v", err)
	}
}
