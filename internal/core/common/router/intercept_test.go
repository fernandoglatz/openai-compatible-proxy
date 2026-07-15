package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// buildTestEngine wires a group the same way Setup does, with stub handlers so the
// routing decision is observable without Mongo or an upstream.
func buildTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	group := engine.Group("/api/v1")
	group.Use(interceptGet("/api/v1/models", func(ginCtx *gin.Context) {
		ginCtx.String(http.StatusOK, "local")
	}))
	group.Any("/*any", func(ginCtx *gin.Context) {
		ginCtx.String(http.StatusOK, "proxy")
	})

	return engine
}

func TestInterceptGetRoutesModelsLocallyAndRestToProxy(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/v1/models", "local"},
		{http.MethodPost, "/api/v1/models", "proxy"},
		{http.MethodPost, "/api/v1/models/load", "proxy"},
		{http.MethodPost, "/api/v1/models/unload", "proxy"},
		{http.MethodPost, "/api/v1/models/download", "proxy"},
		{http.MethodGet, "/api/v1/models/download/status", "proxy"},
		{http.MethodPost, "/api/v1/chat", "proxy"},
	}

	engine := buildTestEngine()

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))

			if recorder.Body.String() != test.want {
				t.Errorf("%s %s handled by %q, want %q", test.method, test.path, recorder.Body.String(), test.want)
			}
		})
	}
}

// Registering a static route alongside a catch-all in one group panics in gin. This
// guards the workaround: the group must build without panicking.
func TestGroupRegistrationDoesNotPanic(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("route registration panicked: %v", recovered)
		}
	}()

	buildTestEngine()
}
