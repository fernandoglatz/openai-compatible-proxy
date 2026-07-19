package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/response"

	"github.com/gin-gonic/gin"
)

// stubModelService returns canned models, standing in for the database.
type stubModelService struct {
	models []entity.Model
}

func (stub *stubModelService) Get(ctx context.Context, id string) (entity.Model, *exceptions.WrappedError) {
	return entity.Model{}, nil
}

func (stub *stubModelService) GetByName(ctx context.Context, name string) (entity.Model, *exceptions.WrappedError) {
	return entity.Model{}, nil
}

func (stub *stubModelService) GetAll(ctx context.Context) ([]entity.Model, *exceptions.WrappedError) {
	return stub.models, nil
}

func (stub *stubModelService) Save(ctx context.Context, model *entity.Model) *exceptions.WrappedError {
	return nil
}

func (stub *stubModelService) Remove(ctx context.Context, model entity.Model) *exceptions.WrappedError {
	return nil
}

// stubLMStudioService reports the upstream as unreachable, which is the case the local
// endpoint exists to serve.
type stubLMStudioService struct{}

func (stub *stubLMStudioService) FetchAndSaveModels(ctx context.Context) *exceptions.WrappedError {
	return &exceptions.WrappedError{Message: "upstream asleep"}
}

func (stub *stubLMStudioService) ProxyRequestStreaming(ctx context.Context, ginCtx *gin.Context, method string, path string, requestBody []byte, headers http.Header) error {
	return nil
}

func TestV1ListModelsServesStoreWhenUpstreamUnreachable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	models := []entity.Model{{
		Name:              "google/gemma-4",
		DisplayName:       "Gemma 4",
		Type:              "llm",
		Publisher:         "google",
		Arch:              "gemma",
		CompatibilityType: "gguf",
		Quantization:      "Q4_K_M",
		SizeBytes:         16000000000,
		ParamsString:      "26B-A4B",
		MaxContextLength:  8192,
		Capabilities:      []string{"completion", "chat", "tools", "vision"},
		LoadedInstanceIDs: []string{"inst-1"},
	}}

	controller := NewLMStudioV1Controller(&stubModelService{models: models}, &stubLMStudioService{})

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)

	controller.ListModels(ginCtx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even though the upstream is unreachable", recorder.Code)
	}

	var body response.LMStudioV1ListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(body.Models) != 1 {
		t.Fatalf("got %d models, want 1", len(body.Models))
	}

	model := body.Models[0]
	if model.Key != "google/gemma-4" {
		t.Errorf("Key = %q, want the entity Name", model.Key)
	}
	if model.ParamsString != "26B-A4B" {
		t.Errorf("ParamsString = %q, want \"26B-A4B\"", model.ParamsString)
	}
	if model.Quantization == nil || model.Quantization.Name != "Q4_K_M" {
		t.Errorf("Quantization = %+v, want name Q4_K_M", model.Quantization)
	}
	if len(model.LoadedInstances) != 1 || model.LoadedInstances[0].ID != "inst-1" {
		t.Errorf("LoadedInstances = %+v, want one instance inst-1", model.LoadedInstances)
	}
	if model.Capabilities == nil || !model.Capabilities.Vision || !model.Capabilities.TrainedForToolUse {
		t.Errorf("Capabilities = %+v, want vision and tool use reconstructed from the stored list", model.Capabilities)
	}
}

func TestV1ListModelsOmitsCapabilitiesForEmbeddings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	models := []entity.Model{{Name: "nomic-embed", Type: "embedding", Capabilities: []string{"embedding"}}}
	controller := NewLMStudioV1Controller(&stubModelService{models: models}, &stubLMStudioService{})

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)

	controller.ListModels(ginCtx)

	var body response.LMStudioV1ListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Models[0].Capabilities != nil {
		t.Errorf("Capabilities = %+v, want nil for an embedding model", body.Models[0].Capabilities)
	}
	if body.Models[0].LoadedInstances == nil {
		t.Error("LoadedInstances is nil; v1 requires an array, so it must serialize as []")
	}
}

// TestV1ListModelsOmitsCapabilitiesForV0SyncedEmbeddingModel exercises the entity shape
// the v0 sync path actually produces: no v1 metadata at all (Capabilities, SizeBytes,
// ParamsString, LoadedInstanceIDs all zero), and Type in whatever vocabulary v0 uses -
// believed to be "embeddings" (plural), unlike v1's "embedding" (singular). Every model in
// The stored row has this shape on a pre-0.4.0 LM Studio host, so this is the case that matters most
// for that fallback to keep working.
func TestV1ListModelsOmitsCapabilitiesForV0SyncedEmbeddingModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	models := []entity.Model{{Name: "nomic-embed-v0", Type: "embeddings"}}
	controller := NewLMStudioV1Controller(&stubModelService{models: models}, &stubLMStudioService{})

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)

	controller.ListModels(ginCtx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var body response.LMStudioV1ListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(body.Models) != 1 {
		t.Fatalf("got %d models, want 1", len(body.Models))
	}

	if body.Models[0].Capabilities != nil {
		t.Errorf("Capabilities = %+v, want nil for a v0-synced embeddings model (Type %q)", body.Models[0].Capabilities, models[0].Type)
	}
	if body.Models[0].LoadedInstances == nil {
		t.Error("LoadedInstances is nil; v1 requires an array, so it must serialize as []")
	}
}

// TestV1ListModelsForV0SyncedVLMModel pins current behavior for a v0-shaped "vlm" entity.
// v1 never emits "vlm" as a Type (its vocabulary has no vision-specific type; vision is a
// capability flag), so this is a v0-only case. With no Capabilities stored, the controller
// cannot tell that a "vlm" model has vision, so it reports a non-nil capabilities object
// with everything false rather than nil - documented here rather than left undefined.
func TestV1ListModelsForV0SyncedVLMModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	models := []entity.Model{{Name: "some-vision-model", Type: "vlm"}}
	controller := NewLMStudioV1Controller(&stubModelService{models: models}, &stubLMStudioService{})

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)

	controller.ListModels(ginCtx)

	var body response.LMStudioV1ListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(body.Models) != 1 {
		t.Fatalf("got %d models, want 1", len(body.Models))
	}

	capabilities := body.Models[0].Capabilities
	if capabilities == nil {
		t.Fatal("Capabilities = nil, want a non-nil object (vlm is not treated as embedding)")
	}
	if capabilities.Vision || capabilities.TrainedForToolUse {
		t.Errorf("Capabilities = %+v, want both false: a v0-shaped entity with no stored Capabilities carries no way to know vlm implies vision", capabilities)
	}
}
