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
