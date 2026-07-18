package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestWriter() (*finishReasonWriter, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	return &finishReasonWriter{ResponseWriter: ginCtx.Writer}, rec
}

func TestFinishReasonWriterDetectsStopAcrossWrites(t *testing.T) {
	w, rec := newTestWriter()
	w.Write([]byte(`data: {"choices":[{"finish_reason":null}]}` + "\n"))
	w.Write([]byte(`data: {"choices":[{"finish_reason":"stop"}]}` + "\n"))
	if !w.done() {
		t.Errorf("done() = false, want true after stop")
	}
	if rec.Body.Len() == 0 {
		t.Errorf("underlying writer received no bytes")
	}
}

func TestFinishReasonWriterHoldsOnToolCalls(t *testing.T) {
	w, _ := newTestWriter()
	w.WriteString(`{"choices":[{"finish_reason":"tool_calls"}]}`)
	if w.done() {
		t.Errorf("done() = true, want false on tool_calls")
	}
}

func TestFinishReasonWriterErrorStatusIsDone(t *testing.T) {
	w, _ := newTestWriter()
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(`{"error":{"message":"boom"}}`))
	if !w.done() {
		t.Errorf("done() = false, want true after error status")
	}
}

func TestFinishReasonWriterResponsesCompletionIsDone(t *testing.T) {
	w, _ := newTestWriter()
	w.Write([]byte(`data: {"type":"response.completed"}` + "\n"))
	if !w.done() {
		t.Errorf("done() = false, want true after response.completed")
	}
}

func TestFinishReasonWriterCapturesViaIoCopy(t *testing.T) {
	w, _ := newTestWriter()
	src := strings.NewReader(`data: {"choices":[{"finish_reason":"stop"}]}` + "\n")
	if _, err := io.Copy(w, src); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	if !w.done() {
		t.Errorf("done() = false after io.Copy of a stop stream, want true")
	}
}
