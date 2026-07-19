package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/scheduler"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

const sessionIDHeader = "X-Session-Id"

// sessionKeyOf identifies the caller's session. opencode sends X-Session-Id on
// every request (one id per session, and each subagent is its own session). Without
// the header, fall back to the client IP so unkeyed callers still map to a key.
func sessionKeyOf(ginCtx *gin.Context) string {
	if id := ginCtx.Request.Header.Get(sessionIDHeader); id != "" {
		return "sid:" + id
	}
	return "ip:" + ginCtx.ClientIP()
}

// isStreamingRequest reports whether the caller asked for a streamed (SSE) response.
// Only those may be heartbeated: an SSE comment is ignored by every compliant parser,
// whereas injecting bytes into a single JSON body would corrupt it. The body is read in
// full and restored, so the proxy controllers downstream still forward it verbatim.
func isStreamingRequest(ginCtx *gin.Context) bool {
	if ginCtx.Request == nil || ginCtx.Request.Body == nil {
		return false
	}

	body, err := io.ReadAll(ginCtx.Request.Body)
	ginCtx.Request.Body.Close()
	if err != nil {
		return false
	}
	ginCtx.Request.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	return payload.Stream
}

// isGatedPath reports whether path should pass through the scheduler. Matching is by
// suffix, so one entry (e.g. "/v1/chat/completions") covers both the OpenAI (/v1/...)
// and LM Studio native (/api/v1/...) namespaces.
func isGatedPath(path string, gated []string) bool {
	for _, g := range gated {
		if strings.HasSuffix(path, g) {
			return true
		}
	}
	return false
}

// SchedulerMiddleware serializes gated generation requests through sched. When the
// scheduler is disabled or the path is not gated, it forwards unchanged.
func SchedulerMiddleware(sched *scheduler.Scheduler, cfg config.SchedulerConfig) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		if !cfg.Enabled || !isGatedPath(ginCtx.Request.URL.Path, cfg.GatedPaths) {
			ginCtx.Next()
			return
		}

		ctx := GetContext(ginCtx)
		key := sessionKeyOf(ginCtx)

		// Start before Acquire: the silence that trips intermediaries begins the moment
		// the request queues, and continues until the upstream's first byte.
		if cfg.HeartbeatInterval > 0 && isStreamingRequest(ginCtx) {
			// A heartbeat commits the response headers before the upstream's are known,
			// so the upstream's Content-Encoding would be dropped while its compressed
			// body is still forwarded verbatim - the client would read gzip as text.
			// Asking upstream for identity encoding keeps the committed headers honest;
			// SSE deltas are small and the hop in front of this service can still
			// compress on its own.
			ginCtx.Request.Header.Del("Accept-Encoding")

			heartbeat := &heartbeatWriter{ResponseWriter: ginCtx.Writer}
			ginCtx.Writer = heartbeat
			defer startHeartbeat(ginCtx.Request.Context(), heartbeat, cfg.HeartbeatAfter, cfg.HeartbeatInterval)()
		}

		release, err := sched.Acquire(ginCtx.Request.Context(), key)
		if err != nil {
			log.Info(ctx).Msg(fmt.Sprintf("Scheduler wait canceled for %s: %v", key, err))
			ginCtx.Abort()
			return
		}

		// The slot can be granted at the same instant the client disconnects: Acquire
		// selects between "granted" and "canceled" and resolves randomly when both are
		// ready, so success here does not mean anyone is still listening. Running the
		// handler would burn a full generation on a departed client while the next
		// session waits, so hand the slot straight back instead.
		if err := ginCtx.Request.Context().Err(); err != nil {
			log.Info(ctx).Msg(fmt.Sprintf("Scheduler slot granted to departed client %s: %v", key, err))
			release(true)
			ginCtx.Abort()
			return
		}

		writer := &finishReasonWriter{ResponseWriter: ginCtx.Writer}
		ginCtx.Writer = writer
		defer func() { release(writer.done()) }()

		ginCtx.Next()
	}
}
