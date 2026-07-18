package controller

import (
	"net/http/httptest"
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
