package controller

import "github.com/gin-gonic/gin"

// finishReasonTailBytes bounds how much of the streamed response is retained. The
// terminal finish_reason is always in the final chunk, so a few KB of tail suffices.
const finishReasonTailBytes = 8192

// finishReasonWriter wraps a gin.ResponseWriter, keeping a bounded tail of the
// streamed bytes so the terminal finish_reason can be read after streaming ends.
// Every other gin.ResponseWriter method is inherited from the embedded writer.
type finishReasonWriter struct {
	gin.ResponseWriter
	tail []byte
}

func (w *finishReasonWriter) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *finishReasonWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *finishReasonWriter) capture(b []byte) {
	w.tail = append(w.tail, b...)
	if len(w.tail) > finishReasonTailBytes {
		w.tail = w.tail[len(w.tail)-finishReasonTailBytes:]
	}
}

// done reports whether the session's turn completed, so a waiting session may
// take over immediately. True when the upstream returned an error status (error
// bodies carry no finish_reason), when the chat/completions finish_reason is
// terminal, or when a Responses-API terminal signal is present.
func (w *finishReasonWriter) done() bool {
	if w.Status() >= 400 {
		return true
	}
	if isSessionDone(lastFinishReason(w.tail)) {
		return true
	}
	return isResponsesDone(w.tail)
}
