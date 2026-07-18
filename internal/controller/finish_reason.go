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
