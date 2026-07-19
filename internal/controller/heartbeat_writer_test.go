package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func newHeartbeatTestWriter() (*heartbeatWriter, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	return &heartbeatWriter{ResponseWriter: ginCtx.Writer}, rec
}

func TestHeartbeatWriterPingOpensSSEStream(t *testing.T) {
	w, rec := newHeartbeatTestWriter()

	if !w.ping() {
		t.Fatalf("ping() = false, want true before any body write")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if !strings.HasPrefix(rec.Body.String(), ":") || !strings.HasSuffix(rec.Body.String(), "\n\n") {
		t.Errorf("body = %q, want an SSE comment line", rec.Body.String())
	}
}

func TestHeartbeatWriterStopsOnceBodyStarts(t *testing.T) {
	w, rec := newHeartbeatTestWriter()
	w.ping()
	before := rec.Body.Len()

	w.Write([]byte("data: {}\n\n"))

	if w.ping() {
		t.Errorf("ping() = true after body started, want false")
	}
	// Nothing beyond the handler's own bytes may reach the client.
	if got := rec.Body.String()[before:]; got != "data: {}\n\n" {
		t.Errorf("bytes after first write = %q, want only the handler payload", got)
	}
}

// A heartbeat commits 200 before the handler knows the upstream status. The real status
// can no longer go on the wire, but finishReasonWriter.done() relies on Status() to
// release the slot immediately on errors, so the intended status must survive.
func TestHeartbeatWriterReportsIntendedStatusAfterCommit(t *testing.T) {
	w, rec := newHeartbeatTestWriter()
	w.ping()

	w.WriteHeader(http.StatusInternalServerError)

	if rec.Code != http.StatusOK {
		t.Errorf("wire status = %d, want 200 (already committed)", rec.Code)
	}
	if w.Status() != http.StatusInternalServerError {
		t.Errorf("Status() = %d, want 500", w.Status())
	}
}

func TestHeartbeatWriterTransparentWithoutPing(t *testing.T) {
	w, rec := newHeartbeatTestWriter()

	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"nope"}`))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 passed through untouched", rec.Code)
	}
	if rec.Body.String() != `{"error":"nope"}` {
		t.Errorf("body = %q, want untouched", rec.Body.String())
	}
}

// The heartbeat runs on its own goroutine while the handler writes on another; a torn
// SSE frame would corrupt the stream for the client.
func TestHeartbeatWriterConcurrentPingAndWriteDoNotInterleave(t *testing.T) {
	w, rec := newHeartbeatTestWriter()
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			w.ping()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			w.Write([]byte("data: {}\n\n"))
		}
	}()
	wg.Wait()

	for _, frame := range strings.SplitAfter(rec.Body.String(), "\n\n") {
		if frame == "" {
			continue
		}
		if !strings.HasPrefix(frame, ":") && frame != "data: {}\n\n" {
			t.Fatalf("torn frame in stream: %q", frame)
		}
	}
}
