package controller

import (
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
