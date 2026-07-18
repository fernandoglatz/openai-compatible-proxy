package controller

import "regexp"

// finishReasonRe matches a quoted finish_reason value in either an SSE chunk
// stream or a single JSON body. A null value has no quotes and is not matched.
var finishReasonRe = regexp.MustCompile(`"finish_reason"\s*:\s*"([^"]+)"`)

// lastFinishReason returns the last quoted finish_reason value in buf, or "".
func lastFinishReason(buf []byte) string {
	matches := finishReasonRe.FindAllSubmatch(buf, -1)
	if len(matches) == 0 {
		return ""
	}
	return string(matches[len(matches)-1][1])
}

// isSessionDone reports whether a finish_reason means the session's turn is
// complete. "tool_calls", unknown values, and "" are treated as not done, so the
// scheduler holds the slot for the grace window in those cases.
func isSessionDone(finishReason string) bool {
	switch finishReason {
	case "stop", "length", "content_filter":
		return true
	default:
		return false
	}
}

// responseEventRe matches the OpenAI Responses API streaming terminal event,
// e.g. {"type":"response.completed"}. responseStatusRe matches the non-streamed
// terminal status, e.g. {"status":"completed"}. The Responses API never emits
// finish_reason, so these are its "turn is done" signals.
var responseEventRe = regexp.MustCompile(`"type"\s*:\s*"response\.(completed|failed|incomplete)"`)
var responseStatusRe = regexp.MustCompile(`"status"\s*:\s*"(completed|failed|incomplete)"`)

// isResponsesDone reports whether buf contains a Responses-API terminal signal.
func isResponsesDone(buf []byte) bool {
	return responseEventRe.Match(buf) || responseStatusRe.Match(buf)
}
