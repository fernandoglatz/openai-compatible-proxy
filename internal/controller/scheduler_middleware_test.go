package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		// Bounded: httptest.Server.Close waits for outstanding requests, so a handler
		// parked forever would turn any failed assertion below into a hung test
		// (t.Fatalf unwinds into the deferred Close) instead of a reported failure.
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
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
	// C must reach the handler. A departed B may still surface first: if the slot is
	// granted at the same instant its client cancels, Acquire's select between w.ready
	// and ctx.Done resolves randomly, so B's handler can legitimately run once and then
	// release. That is a wasted turn, not a leak - what must never happen is C being
	// starved, so skip a stale B and keep waiting for C.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case k := <-started:
			if k == "C" {
				goto acquired
			}
			t.Logf("stale session %q won the grant/cancel race and took a turn", k)
			release <- struct{}{} // let it finish so the slot moves on to C
		case <-deadline:
			t.Fatal("C never acquired the slot: queued B was not removed on disconnect (slot leaked)")
		}
	}
acquired:
	release <- struct{}{}
	if err := <-cResult; err != nil {
		t.Fatalf("C request failed: %v", err)
	}
}

func TestIsStreamingRequestPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"m","stream":true,"messages":[]}`
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))

	if !isStreamingRequest(ginCtx) {
		t.Errorf("isStreamingRequest = false, want true")
	}

	// The handler downstream must still see the untouched body.
	got, err := io.ReadAll(ginCtx.Request.Body)
	if err != nil {
		t.Fatalf("re-reading body failed: %v", err)
	}
	if string(got) != body {
		t.Errorf("body after detection = %q, want %q", got, body)
	}
}

func TestIsStreamingRequestFalseCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := map[string]string{
		"explicit false": `{"model":"m","stream":false}`,
		"absent":         `{"model":"m"}`,
		"not json":       `garbage`,
		"empty":          ``,
	}
	for name, body := range cases {
		ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		if isStreamingRequest(ginCtx) {
			t.Errorf("%s: isStreamingRequest = true, want false", name)
		}
	}
}

func heartbeatTestEngine(t *testing.T, cfg config.SchedulerConfig, handler gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(SchedulerMiddleware(scheduler.NewScheduler(10*time.Millisecond), cfg))
	engine.POST("/v1/chat/completions", handler)
	return engine
}

func streamingRequest(sid string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"m","stream":true}`))
	req.Header.Set("X-Session-Id", sid)
	return req
}

// A slot may be granted long before the upstream emits its first token (prompt
// processing on a large context), so the heartbeat must cover that window too, not
// just the queue wait.
func TestSchedulerMiddlewareHeartbeatsWhileAwaitingFirstByte(t *testing.T) {
	cfg := config.SchedulerConfig{
		Enabled:           true,
		GatedPaths:        []string{"/v1/chat/completions"},
		HeartbeatAfter:    10 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
	}
	engine := heartbeatTestEngine(t, cfg, func(c *gin.Context) {
		time.Sleep(120 * time.Millisecond)
		c.String(http.StatusOK, `data: {"choices":[{"finish_reason":"stop"}]}`)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, streamingRequest("ses_slow"))

	body := rec.Body.String()
	if !strings.HasPrefix(body, heartbeatComment) {
		t.Errorf("body did not start with a heartbeat: %q", body)
	}
	if strings.Count(body, heartbeatComment) < 2 {
		t.Errorf("want repeated heartbeats, got %d in %q", strings.Count(body, heartbeatComment), body)
	}
	if !strings.HasSuffix(body, `data: {"choices":[{"finish_reason":"stop"}]}`) {
		t.Errorf("handler payload missing or not last: %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

// The queued request is the case that was timing out: it sends nothing at all until
// the active session finishes.
func TestSchedulerMiddlewareHeartbeatsWhileQueued(t *testing.T) {
	cfg := config.SchedulerConfig{
		Enabled:           true,
		GatedPaths:        []string{"/v1/chat/completions"},
		HeartbeatAfter:    10 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
	}
	release := make(chan struct{})
	engine := heartbeatTestEngine(t, cfg, func(c *gin.Context) {
		if c.Request.Header.Get("X-Session-Id") == "ses_holder" {
			<-release
		}
		c.String(http.StatusOK, `data: {"choices":[{"finish_reason":"stop"}]}`)
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		engine.ServeHTTP(httptest.NewRecorder(), streamingRequest("ses_holder"))
	}()

	time.Sleep(20 * time.Millisecond) // let the holder take the slot
	queued := httptest.NewRecorder()
	var qwg sync.WaitGroup
	qwg.Add(1)
	go func() {
		defer qwg.Done()
		engine.ServeHTTP(queued, streamingRequest("ses_queued"))
	}()

	time.Sleep(80 * time.Millisecond) // queued request is stuck waiting for the slot
	close(release)
	wg.Wait()
	qwg.Wait()

	if !strings.HasPrefix(queued.Body.String(), heartbeatComment) {
		t.Errorf("queued request sent no heartbeat while waiting: %q", queued.Body.String())
	}
}

func TestSchedulerMiddlewareNoHeartbeatForNonStreaming(t *testing.T) {
	cfg := config.SchedulerConfig{
		Enabled:           true,
		GatedPaths:        []string{"/v1/chat/completions"},
		HeartbeatAfter:    10 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
	}
	engine := heartbeatTestEngine(t, cfg, func(c *gin.Context) {
		time.Sleep(60 * time.Millisecond)
		c.String(http.StatusOK, `{"choices":[{"finish_reason":"stop"}]}`)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"m","stream":false}`))
	req.Header.Set("X-Session-Id", "ses_json")
	engine.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), ":") && strings.HasPrefix(rec.Body.String(), ":") {
		t.Errorf("non-streaming body was polluted with a heartbeat: %q", rec.Body.String())
	}
	if rec.Body.String() != `{"choices":[{"finish_reason":"stop"}]}` {
		t.Errorf("body = %q, want untouched JSON", rec.Body.String())
	}
}

func TestSchedulerMiddlewareHeartbeatDisabledByZeroInterval(t *testing.T) {
	cfg := config.SchedulerConfig{
		Enabled:           true,
		GatedPaths:        []string{"/v1/chat/completions"},
		HeartbeatAfter:    10 * time.Millisecond,
		HeartbeatInterval: 0,
	}
	engine := heartbeatTestEngine(t, cfg, func(c *gin.Context) {
		time.Sleep(60 * time.Millisecond)
		c.String(http.StatusOK, `data: {"choices":[{"finish_reason":"stop"}]}`)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, streamingRequest("ses_off"))

	if strings.Contains(rec.Body.String(), heartbeatComment) {
		t.Errorf("heartbeat emitted despite zero interval: %q", rec.Body.String())
	}
}

// A heartbeat commits the response headers before the upstream's are known, so the
// upstream's Content-Encoding is lost while its compressed body is still forwarded -
// the client would then read gzip bytes as text. Asking upstream for identity encoding
// is what keeps the committed headers truthful.
func TestSchedulerMiddlewareDropsAcceptEncodingWhenHeartbeating(t *testing.T) {
	cfg := config.SchedulerConfig{
		Enabled:           true,
		GatedPaths:        []string{"/v1/chat/completions"},
		HeartbeatAfter:    10 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
	}
	var seen string
	engine := heartbeatTestEngine(t, cfg, func(c *gin.Context) {
		seen = c.Request.Header.Get("Accept-Encoding")
		c.String(http.StatusOK, `data: {"choices":[{"finish_reason":"stop"}]}`)
	})

	req := streamingRequest("ses_gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "" {
		t.Errorf("Accept-Encoding forwarded upstream = %q, want it removed", seen)
	}
}

// Without a heartbeat the upstream's own headers reach the client untouched, so
// compression must keep working.
func TestSchedulerMiddlewareKeepsAcceptEncodingWhenNotHeartbeating(t *testing.T) {
	cfg := config.SchedulerConfig{
		Enabled:           true,
		GatedPaths:        []string{"/v1/chat/completions"},
		HeartbeatInterval: 0,
	}
	var seen string
	engine := heartbeatTestEngine(t, cfg, func(c *gin.Context) {
		seen = c.Request.Header.Get("Accept-Encoding")
		c.String(http.StatusOK, `data: {"choices":[{"finish_reason":"stop"}]}`)
	})

	req := streamingRequest("ses_plain")
	req.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "gzip" {
		t.Errorf("Accept-Encoding = %q, want gzip preserved", seen)
	}
}

// A client can disconnect while queued, and the slot may still be granted to it: the
// scheduler's select between "granted" and "canceled" resolves randomly when both are
// ready. Running the handler then burns a full generation on a client that will never
// read it, while the next session waits. The slot must be handed straight back.
func TestSchedulerMiddlewareSkipsHandlerForCanceledRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	cfg := config.SchedulerConfig{Enabled: true, GatedPaths: []string{"/v1/chat/completions"}}
	engine.Use(SchedulerMiddleware(scheduler.NewScheduler(10*time.Millisecond), cfg))

	var handlerRan atomic.Bool
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerRan.Store(true)
		c.String(http.StatusOK, `{"choices":[{"finish_reason":"stop"}]}`)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the client is already gone when the slot is granted
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	req.Header.Set("X-Session-Id", "departed")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	if handlerRan.Load() {
		t.Error("handler ran for a request whose client had already disconnected")
	}

	// The slot must not be stuck: a live session still gets served.
	served := make(chan struct{})
	go func() {
		defer close(served)
		live := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		live.Header.Set("X-Session-Id", "live")
		engine.ServeHTTP(httptest.NewRecorder(), live)
	}()
	select {
	case <-served:
	case <-time.After(2 * time.Second):
		t.Fatal("slot leaked: a live session could not acquire it")
	}
}
