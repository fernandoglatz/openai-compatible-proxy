package controller

import (
	"context"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// heartbeatComment is an SSE comment line: compliant parsers discard it, so it keeps
// the connection alive without appearing as a chunk to the client.
const heartbeatComment = ": scheduler queued\n\n"

// heartbeatWriter lets a background goroutine hold a streamed connection open while the
// request waits for a scheduler slot and for the upstream's first byte. Intermediaries
// (CloudFront defaults to 30s) abort a response that sends nothing for too long, and a
// queued request is silent by nature.
//
// Emitting a heartbeat commits the response: status 200 and the SSE headers go on the
// wire before the upstream status is known. Any status the handler sets afterwards can
// no longer be sent, so it is recorded and reported by Status() instead - finishReasonWriter
// reads it to release the slot promptly on upstream errors.
//
// ping and the handler's writes run on different goroutines, so every mutation is held
// under mu for the duration of the underlying write; a heartbeat interleaved into a
// half-written SSE frame would corrupt the stream.
type heartbeatWriter struct {
	gin.ResponseWriter

	mu             sync.Mutex
	committed      bool // heartbeat has written the SSE headers
	bodyStarted    bool // the handler has written real bytes; heartbeats must stop
	intendedStatus int  // status the handler set after the heartbeat committed
}

// ping emits one heartbeat, opening the SSE stream on first use. It reports whether
// heartbeating should continue: false once the handler owns the stream.
func (w *heartbeatWriter) ping() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.bodyStarted {
		return false
	}
	if !w.committed {
		header := w.ResponseWriter.Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		w.ResponseWriter.WriteHeader(200)
		w.committed = true
	}

	if _, err := w.ResponseWriter.WriteString(heartbeatComment); err != nil {
		return false
	}
	w.ResponseWriter.Flush()
	return true
}

// startHeartbeat pings w every interval, beginning only after the request has already
// been silent for after - a fast request completes untouched, keeping its real HTTP
// status. It returns a stop function that blocks until the goroutine has exited, so no
// heartbeat can land on the response after the handler is done with it.
func startHeartbeat(ctx context.Context, w *heartbeatWriter, after time.Duration, interval time.Duration) func() {
	done := make(chan struct{})
	exited := make(chan struct{})

	go func() {
		defer close(exited)

		grace := time.NewTimer(after)
		defer grace.Stop()
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-grace.C:
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if !w.ping() {
				return // the handler owns the stream now
			}
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
		<-exited
	}
}

func (w *heartbeatWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bodyStarted = true
	return w.ResponseWriter.Write(b)
}

func (w *heartbeatWriter) WriteString(s string) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bodyStarted = true
	return w.ResponseWriter.WriteString(s)
}

// WriteHeader records the handler's status and forwards it only if no heartbeat has
// already committed the response, which would make a second write-header a no-op plus
// a gin warning.
func (w *heartbeatWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.intendedStatus = code
	if w.committed {
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

// Status reports the status the handler intended, which differs from the one on the
// wire only when a heartbeat committed 200 first.
func (w *heartbeatWriter) Status() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.committed && w.intendedStatus != 0 {
		return w.intendedStatus
	}
	return w.ResponseWriter.Status()
}
