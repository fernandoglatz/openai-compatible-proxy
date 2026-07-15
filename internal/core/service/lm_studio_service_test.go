package service

import (
	"fernandoglatz/openai-compatible-proxy/internal/core/model/dto"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/api"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

// proxyAndCaptureUpstreamAuth proxies a request carrying clientAuthorization and
// returns the Authorization header LM Studio actually received.
func proxyAndCaptureUpstreamAuth(t *testing.T, lmStudioAPIKey string, clientAuthorization string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)

	upstreamAuthorization := ""
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		upstreamAuthorization = req.Header.Get("Authorization")
		writer.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	config.ApplicationConfig.LMStudio.URL = upstream.URL
	config.ApplicationConfig.LMStudio.APIKey = lmStudioAPIKey

	service := NewLMStudioService(api.NewLMStudioAPI(), nil)

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	headers := http.Header{}
	headers.Set("Authorization", clientAuthorization)
	headers.Set("X-Custom-Header", "should-be-forwarded")

	err := service.ProxyRequestStreaming(ginCtx.Request.Context(), ginCtx, http.MethodPost, "/v1/chat/completions", []byte("{}"), headers)
	if err != nil {
		t.Fatalf("ProxyRequestStreaming returned error: %v", err)
	}

	return upstreamAuthorization
}

// TestProxyPreservesQueryString guards GET /api/v1/models/download/status, which
// identifies its job by query parameter: dropping the query string silently breaks it.
func TestProxyPreservesQueryString(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamRawQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		upstreamRawQuery = req.URL.RawQuery
		writer.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	config.ApplicationConfig.LMStudio.URL = upstream.URL
	config.ApplicationConfig.LMStudio.APIKey = ""

	service := NewLMStudioService(api.NewLMStudioAPI(), nil)

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/models/download/status?jobId=abc%2Fdef&other=1", nil)

	err := service.ProxyRequestStreaming(ginCtx.Request.Context(), ginCtx, http.MethodGet, "/api/v1/models/download/status", nil, http.Header{})
	if err != nil {
		t.Fatalf("ProxyRequestStreaming returned error: %v", err)
	}

	if want := "jobId=abc%2Fdef&other=1"; upstreamRawQuery != want {
		t.Errorf("upstream got RawQuery %q, want %q", upstreamRawQuery, want)
	}
}

// TestProxyOmitsQueryStringWhenAbsent ensures no stray "?" is appended when the
// incoming request carries no query string.
func TestProxyOmitsQueryStringWhenAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamRequestURI string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		upstreamRequestURI = req.RequestURI
		writer.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	config.ApplicationConfig.LMStudio.URL = upstream.URL
	config.ApplicationConfig.LMStudio.APIKey = ""

	service := NewLMStudioService(api.NewLMStudioAPI(), nil)

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)

	err := service.ProxyRequestStreaming(ginCtx.Request.Context(), ginCtx, http.MethodGet, "/api/v1/models", nil, http.Header{})
	if err != nil {
		t.Fatalf("ProxyRequestStreaming returned error: %v", err)
	}

	if want := "/api/v1/models"; upstreamRequestURI != want {
		t.Errorf("upstream got RequestURI %q, want %q (no trailing '?')", upstreamRequestURI, want)
	}
}

func TestProxyReplacesClientTokenWithLMStudioKey(t *testing.T) {
	upstreamAuthorization := proxyAndCaptureUpstreamAuth(t, "lm-studio-key", "Bearer client-proxy-token")

	if want := "Bearer lm-studio-key"; upstreamAuthorization != want {
		t.Errorf("upstream got Authorization %q, want %q", upstreamAuthorization, want)
	}
}

func TestProxyDropsClientTokenWhenNoLMStudioKeyConfigured(t *testing.T) {
	upstreamAuthorization := proxyAndCaptureUpstreamAuth(t, "", "Bearer client-proxy-token")

	if upstreamAuthorization != "" {
		t.Errorf("upstream got Authorization %q, want it dropped entirely", upstreamAuthorization)
	}
}

func TestConvertToInternalModelCarriesV1Metadata(t *testing.T) {
	service := NewLMStudioService(nil, nil)

	model := service.convertToInternalModel(dto.LMStudioModel{
		ID:                "google/gemma-4",
		Object:            "model",
		Type:              "llm",
		Publisher:         "google",
		Arch:              "gemma",
		CompatibilityType: "gguf",
		Quantization:      "Q4_K_M",
		State:             "loaded",
		MaxContextLength:  8192,
		DisplayName:       "Gemma 4",
		SizeBytes:         16000000000,
		ParamsString:      "26B-A4B",
		Capabilities:      []string{"completion", "chat", "tools"},
		LoadedInstanceIDs: []string{"inst-1"},
	})

	if model.Name != "google/gemma-4" {
		t.Errorf("Name = %q, want the DTO ID", model.Name)
	}
	if model.DisplayName != "Gemma 4" {
		t.Errorf("DisplayName = %q, want \"Gemma 4\"", model.DisplayName)
	}
	if model.SizeBytes != 16000000000 {
		t.Errorf("SizeBytes = %d, want 16000000000", model.SizeBytes)
	}
	if model.ParamsString != "26B-A4B" {
		t.Errorf("ParamsString = %q, want \"26B-A4B\"", model.ParamsString)
	}
	if !reflect.DeepEqual(model.Capabilities, []string{"completion", "chat", "tools"}) {
		t.Errorf("Capabilities = %v, want [completion chat tools]", model.Capabilities)
	}
	if !reflect.DeepEqual(model.LoadedInstanceIDs, []string{"inst-1"}) {
		t.Errorf("LoadedInstanceIDs = %v, want [inst-1]", model.LoadedInstanceIDs)
	}
}

// The v0 sync path supplies none of the v1 metadata; it must degrade to zero values
// rather than failing, since consumers fall back on emptiness.
func TestConvertToInternalModelV0PathLeavesV1MetadataEmpty(t *testing.T) {
	service := NewLMStudioService(nil, nil)

	model := service.convertToInternalModel(dto.LMStudioModel{ID: "m", Type: "llm", State: "loaded"})

	if model.DisplayName != "" || model.SizeBytes != 0 || model.ParamsString != "" {
		t.Errorf("v1 metadata is non-zero on the v0 path: %+v", model)
	}
	if len(model.Capabilities) != 0 || len(model.LoadedInstanceIDs) != 0 {
		t.Errorf("v1 slices are non-empty on the v0 path: %+v", model)
	}
}
