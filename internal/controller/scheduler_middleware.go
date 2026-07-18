package controller

import (
	"fmt"
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

		release, err := sched.Acquire(ginCtx.Request.Context(), key)
		if err != nil {
			log.Info(ctx).Msg(fmt.Sprintf("Scheduler wait canceled for %s: %v", key, err))
			ginCtx.Abort()
			return
		}

		writer := &finishReasonWriter{ResponseWriter: ginCtx.Writer}
		ginCtx.Writer = writer

		ginCtx.Next()

		release(writer.done())
	}
}
