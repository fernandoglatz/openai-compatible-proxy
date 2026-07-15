package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
)

// A non-streaming completion sends no response headers until generation finishes.
// Any ResponseHeaderTimeout therefore caps generation time regardless of
// lm-studio.timeout, which breaks vision and long outputs.
func TestTransportHasNoResponseHeaderTimeout(t *testing.T) {
	config.ApplicationConfig.LMStudio.URL = "http://localhost:1234"
	config.ApplicationConfig.LMStudio.Timeout = 600 * time.Second

	client := NewLMStudioAPI()

	transport, ok := client.GetHTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.GetHTTPClient().Transport)
	}

	if transport.ResponseHeaderTimeout != 0 {
		t.Errorf("ResponseHeaderTimeout is %v; must be 0 or it caps generation time", transport.ResponseHeaderTimeout)
	}

	if client.GetHTTPClient().Timeout != 600*time.Second {
		t.Errorf("client timeout is %v, want lm-studio.timeout (600s)", client.GetHTTPClient().Timeout)
	}
}

// Guards the fix behaviourally: generation slower than the old 10s cap must survive.
func TestSlowGenerationSurvives(t *testing.T) {
	if testing.Short() {
		t.Skip("takes ~12s; run without -short")
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(12 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer upstream.Close()

	config.ApplicationConfig.LMStudio.URL = upstream.URL
	config.ApplicationConfig.LMStudio.Timeout = 600 * time.Second

	client := NewLMStudioAPI()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, upstream.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.DoRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("slow generation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want 200", resp.StatusCode)
	}
}

// newVersionedUpstream serves /api/v1/models and /api/v0/models with the given statuses
// and bodies, recording which paths were actually requested.
func newVersionedUpstream(v1Status int, v1Body string, v0Status int, v0Body string) (*httptest.Server, func() []string) {
	var mutex sync.Mutex
	var requested []string

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		mutex.Lock()
		requested = append(requested, req.URL.Path)
		mutex.Unlock()

		writer.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(req.URL.Path, "/api/v1/models") {
			writer.WriteHeader(v1Status)
			writer.Write([]byte(v1Body))
			return
		}

		writer.WriteHeader(v0Status)
		writer.Write([]byte(v0Body))
	}))

	return server, func() []string {
		mutex.Lock()
		defer mutex.Unlock()
		return requested
	}
}

const v1ModelsBody = `{"models":[{"key":"m-v1","display_name":"M","type":"llm","publisher":"p",
  "architecture":"llama","quantization":{"name":"Q4_K_M","bits_per_weight":4},"format":"gguf",
  "size_bytes":42,"params_string":"7B","max_context_length":4096,"loaded_instances":[],
  "capabilities":{"vision":false,"trained_for_tool_use":true}}]}`

const v0ModelsBody = `{"object":"list","data":[{"id":"m-v0","object":"model","type":"llm",
  "publisher":"p","arch":"llama","compatibility_type":"gguf","quantization":"Q4_K_M",
  "state":"loaded","max_context_length":4096}]}`

func configureClient(upstreamURL string) *LMStudioAPI {
	config.ApplicationConfig.LMStudio.URL = upstreamURL
	config.ApplicationConfig.LMStudio.APIKey = ""
	config.ApplicationConfig.LMStudio.Timeout = 10 * time.Second
	return NewLMStudioAPI()
}

func TestGetModelsPrefersV1AndDoesNotCallV0(t *testing.T) {
	upstream, requested := newVersionedUpstream(http.StatusOK, v1ModelsBody, http.StatusOK, v0ModelsBody)
	defer upstream.Close()

	models, err := configureClient(upstream.URL).GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels returned error: %v", err)
	}

	if len(models) != 1 || models[0].ID != "m-v1" {
		t.Fatalf("got models %+v, want the single v1 model m-v1", models)
	}
	if models[0].ParamsString != "7B" {
		t.Errorf("ParamsString = %q, want \"7B\" — v1 metadata must survive normalization", models[0].ParamsString)
	}
	for _, path := range requested() {
		if strings.HasSuffix(path, "/api/v0/models") {
			t.Error("v0 was called even though v1 succeeded")
		}
	}
}

func TestGetModelsFallsBackToV0WhenV1Absent(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream, requested := newVersionedUpstream(status, `{}`, http.StatusOK, v0ModelsBody)
			defer upstream.Close()

			models, err := configureClient(upstream.URL).GetModels(context.Background())
			if err != nil {
				t.Fatalf("GetModels returned error: %v", err)
			}

			if len(models) != 1 || models[0].ID != "m-v0" {
				t.Fatalf("got models %+v, want the single v0 model m-v0", models)
			}

			calledV0 := false
			for _, path := range requested() {
				if strings.HasSuffix(path, "/api/v0/models") {
					calledV0 = true
				}
			}
			if !calledV0 {
				t.Errorf("v0 was never called despite v1 returning %d", status)
			}
		})
	}
}

// A sleeping host fails both endpoints. Falling through would double the dial wait and
// log a misleading "v1 unsupported" cause, so connection errors must propagate as-is.
//
// This is asserted by counting TCP accepts rather than recorded HTTP paths: a listener
// that accepts and immediately closes the connection (no response written) guarantees
// every attempt is a genuine connection-level failure, and a second accept can only mean
// GetModels dialed again to retry against /api/v0/models.
func TestGetModelsDoesNotFallBackOnConnectionError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}

	var mutex sync.Mutex
	acceptCount := 0

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}

			mutex.Lock()
			acceptCount++
			mutex.Unlock()

			conn.Close()
		}
	}()

	upstreamURL := "http://" + listener.Addr().String()

	_, err = configureClient(upstreamURL).GetModels(context.Background())
	if err == nil {
		t.Fatal("GetModels succeeded against a connection that accepts and closes immediately, want a connection error")
	}

	listener.Close()
	<-done // wait for the accept loop to exit before reading acceptCount

	mutex.Lock()
	got := acceptCount
	mutex.Unlock()

	if got != 1 {
		t.Errorf("accept count = %d, want 1 — a second accept means GetModels dialed again for the v0 fallback", got)
	}
}

func TestGetModelsErrorsWhenBothVersionsAbsent(t *testing.T) {
	upstream, _ := newVersionedUpstream(http.StatusNotFound, `{}`, http.StatusNotFound, `{}`)
	defer upstream.Close()

	if _, err := configureClient(upstream.URL).GetModels(context.Background()); err == nil {
		t.Fatal("GetModels succeeded with both versions absent, want an error")
	}
}
