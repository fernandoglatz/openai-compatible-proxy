package controller

import (
	"context"
	"encoding/json"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/response"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupAuthEngine(apiKeys []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	config.ApplicationConfig.OpenAI.APIKeys = apiKeys

	engine := gin.New()
	engine.Use(TraceMiddleware())
	engine.Use(AuthenticationMiddleware(context.Background()))
	engine.GET("/v1/models", func(ginCtx *gin.Context) {
		ginCtx.String(http.StatusOK, "reached handler")
	})

	return engine
}

func doAuthRequest(engine *gin.Engine, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	return recorder
}

func TestAuthenticationMiddleware(t *testing.T) {
	apiKeys := []string{"token-one", "token-two", "token-three"}

	tests := []struct {
		name          string
		authorization string
		expected      int
	}{
		{"first configured token", "Bearer token-one", http.StatusOK},
		{"middle configured token", "Bearer token-two", http.StatusOK},
		{"last configured token", "Bearer token-three", http.StatusOK},
		{"lowercase bearer scheme", "bearer token-one", http.StatusOK},
		{"surrounding whitespace", "Bearer  token-one ", http.StatusOK},
		{"unknown token", "Bearer token-unknown", http.StatusUnauthorized},
		{"token is a prefix of a valid one", "Bearer token-on", http.StatusUnauthorized},
		{"missing header", "", http.StatusUnauthorized},
		{"empty bearer token", "Bearer ", http.StatusUnauthorized},
		{"bearer scheme only", "Bearer", http.StatusUnauthorized},
		{"wrong scheme", "Basic token-one", http.StatusUnauthorized},
		{"raw token without scheme", "token-one", http.StatusUnauthorized},
	}

	engine := setupAuthEngine(apiKeys)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := doAuthRequest(engine, test.authorization)

			if recorder.Code != test.expected {
				t.Errorf("Authorization %q: got status %d, want %d", test.authorization, recorder.Code, test.expected)
			}
		})
	}
}

func TestAuthenticationMiddlewareRejectionIsOpenAIFormatted(t *testing.T) {
	engine := setupAuthEngine([]string{"token-one"})
	recorder := doAuthRequest(engine, "Bearer nope")

	var body response.OpenAIErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if body.Error.Code != "invalid_api_key" {
		t.Errorf("got error code %q, want %q", body.Error.Code, "invalid_api_key")
	}

	if body.Error.Type != "invalid_request_error" {
		t.Errorf("got error type %q, want %q", body.Error.Type, "invalid_request_error")
	}
}

func TestAuthenticationMiddlewareRejectionDoesNotLeakTokens(t *testing.T) {
	engine := setupAuthEngine([]string{"super-secret-token"})
	recorder := doAuthRequest(engine, "Bearer nope")

	if body := recorder.Body.String(); strings.Contains(body, "super-secret-token") {
		t.Errorf("response body leaked a configured token: %s", body)
	}
}

// With no tokens configured the proxy stays open, so existing deployments keep
// working after an upgrade that predates the openai.api-keys setting.
func TestAuthenticationMiddlewareDisabledWhenNoKeysConfigured(t *testing.T) {
	engine := setupAuthEngine([]string{})

	for _, authorization := range []string{"", "Bearer anything"} {
		recorder := doAuthRequest(engine, authorization)

		if recorder.Code != http.StatusOK {
			t.Errorf("Authorization %q: got status %d, want %d", authorization, recorder.Code, http.StatusOK)
		}
	}
}

// setupPathTraversalEngine wires PathTraversalMiddleware ahead of a wildcard route that
// always succeeds, so the recorded status reflects only the middleware's decision.
func setupPathTraversalEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.RedirectTrailingSlash = false
	engine.Use(PathTraversalMiddleware())
	engine.Any("/*any", func(ginCtx *gin.Context) {
		ginCtx.String(http.StatusOK, "reached")
	})

	return engine
}

// TestPathTraversalMiddleware guards the fix for dot-segment path traversal: gin matches
// routes on the literal (decoded) URL.Path without cleaning it, and the proxy controllers
// forward that path verbatim upstream. A request like /api/v0/../v1/models/load could
// therefore match the open /api/v0 group while actually targeting the authenticated
// /api/v1 group upstream. Every case here must reach the middleware with dot-segments
// still in Path - httptest.NewRequest parses the URL the same way gin's server does,
// decoding %2e%2e to ".." before routing ever sees it.
func TestPathTraversalMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"dot-segment traversal", "/api/v0/../v1/models/load", http.StatusBadRequest},
		{"encoded dot-segment traversal", "/api/v0/%2e%2e/v1/models/load", http.StatusBadRequest},
		{"double-encoded dot-segment traversal", "/api/v0/%252e%252e/v1/models/load", http.StatusBadRequest},
		{"triple-encoded dot-segment traversal", "/api/v0/%25252e%25252e/v1/models/load", http.StatusBadRequest},
		{"clean v1 models path passes through", "/api/v1/models", http.StatusOK},
		{"trailing slash on models is tolerated", "/api/v1/models/", http.StatusOK},
		{"clean v1 chat completions path passes through", "/v1/chat/completions", http.StatusOK},
		{"model key with literal slash passes through", "/api/v0/models/google/gemma-4-26b-a4b", http.StatusOK},
		{"model key with encoded slash passes through", "/api/v0/models/google%2Fgemma-4-26b-a4b", http.StatusOK},
	}

	engine := setupPathTraversalEngine()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)

			if recorder.Code != test.wantStatus {
				t.Errorf("path %q: got status %d, want %d (body: %s)", test.path, recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestIsCleanRequestPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/models", true},
		{"/api/v1/models/", true},
		{"/", true},
		{"/api/v0/../v1/models/load", false},
		{"/api/v0/../v1/models/load/", false},
		{"/..", false},
		{"/api//models", false},
		{"", false},
		{"relative/path", false},
		// %2e%2e is what URL.Path holds once gin/net-http has decoded a single
		// %252e%252e layer sent by the client; a second unescape reveals the
		// dot-segments the proxy would still forward upstream (still encoded).
		{"/api/v0/%2e%2e/v1/models/load", false},
		{"/api/v0/models/google/gemma-4-26b-a4b", true},
		// Both loop exits must reject rather than assume clean. A path still decoding
		// past the bound, and one whose escape is malformed, are each treated as dirty.
		{"/api/v0/%2525252e%2525252e/v1/models/load", false},
		{"/api/v0/%2e%2e%zz/v1/models/load", false},
		// A model key may legitimately contain encoded dots; only a whole "." or ".."
		// segment is a traversal.
		{"/api/v0/models/some.model.name", true},
		{"/api/v0/models/org/model-v1.2.3", true},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := isCleanRequestPath(test.path); got != test.want {
				t.Errorf("isCleanRequestPath(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}
