package controller

import (
	"net/http"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/response"
	"fernandoglatz/openai-compatible-proxy/internal/core/port/service"

	"github.com/gin-gonic/gin"
)

// Capability names as stored on entity.Model, in Ollama's vocabulary.
const (
	CAPABILITY_TOOLS  = "tools"
	CAPABILITY_VISION = "vision"
)

// TYPE_EMBEDDING is the type string LM Studio v1 emits for embedding models, and also
// the single entry the v1 normalizer stores in Capabilities for them (see
// normalizeV1Capabilities in lm_studio_v1.go). TYPE_EMBEDDING_PLURAL is the spelling
// LM Studio v0 is believed to emit on its "type" field instead - v0 predates the
// Capabilities list entirely, so on that path Type is all that is available.
const (
	TYPE_EMBEDDING        = "embedding"
	TYPE_EMBEDDING_PLURAL = "embeddings"
)

type LMStudioV1Controller struct {
	modelService    service.IModelService
	lmStudioService service.ILMStudioService
}

func NewLMStudioV1Controller(modelService service.IModelService, lmStudioService service.ILMStudioService) *LMStudioV1Controller {
	return &LMStudioV1Controller{
		modelService:    modelService,
		lmStudioService: lmStudioService,
	}
}

// @Tags	lm-studio
// @Summary	List available models (native v1 API)
// @Produce	json
// @Success	200	{object}	response.LMStudioV1ListResponse
// @Router	/api/v1/models [get]
func (controller *LMStudioV1Controller) ListModels(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)

	// Try to refresh from LM Studio, but continue even if it fails: serving the stored
	// models while the host is asleep is the reason this endpoint is not proxied.
	err := controller.lmStudioService.FetchAndSaveModels(ctx)
	if err != nil {
		log.Warn(ctx).Msg("LM Studio API not available or error fetching models, returning only database models")
	}

	log.Info(ctx).Msg("Listing LM Studio v1 models")

	models, err := controller.modelService.GetAll(ctx)
	if err != nil {
		log.Error(ctx).Msg("Failed to get models")
		ginCtx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch models"})
		return
	}

	ginCtx.JSON(http.StatusOK, controller.convertToLMStudioV1ListResponse(models))
}

func (controller *LMStudioV1Controller) convertToLMStudioV1ListResponse(models []entity.Model) response.LMStudioV1ListResponse {
	v1Models := make([]response.LMStudioV1Model, constants.ZERO, len(models))

	for _, model := range models {
		v1Models = append(v1Models, controller.convertToLMStudioV1Model(model))
	}

	return response.LMStudioV1ListResponse{
		Models: v1Models,
	}
}

func (controller *LMStudioV1Controller) convertToLMStudioV1Model(model entity.Model) response.LMStudioV1Model {
	instances := make([]response.LMStudioV1LoadedInstance, constants.ZERO, len(model.LoadedInstanceIDs))
	for _, instanceID := range model.LoadedInstanceIDs {
		instances = append(instances, response.LMStudioV1LoadedInstance{ID: instanceID})
	}

	v1Model := response.LMStudioV1Model{
		Key:              model.Name,
		DisplayName:      model.DisplayName,
		Type:             model.Type,
		Publisher:        model.Publisher,
		Architecture:     model.Arch,
		Format:           model.CompatibilityType,
		SizeBytes:        model.SizeBytes,
		ParamsString:     model.ParamsString,
		MaxContextLength: model.MaxContextLength,
		LoadedInstances:  instances,
		Capabilities:     controller.convertToV1Capabilities(model),
	}

	if model.Quantization != "" {
		v1Model.Quantization = &response.LMStudioV1Quantization{Name: model.Quantization}
	}

	return v1Model
}

// convertToV1Capabilities rebuilds the v1 capabilities object from the stored Ollama
// vocabulary. reasoning is not reconstructed: the stored list cannot express its options.
func (controller *LMStudioV1Controller) convertToV1Capabilities(model entity.Model) *response.LMStudioV1Capabilities {
	if isEmbeddingModel(model) {
		return nil
	}

	capabilities := &response.LMStudioV1Capabilities{}

	for _, capability := range model.Capabilities {
		switch capability {
		case CAPABILITY_VISION:
			capabilities.Vision = true
		case CAPABILITY_TOOLS:
			capabilities.TrainedForToolUse = true
		}
	}

	return capabilities
}

// isEmbeddingModel decides whether model is an embedding model.
//
// When Capabilities is populated, it is authoritative: the v1 normalizer
// (normalizeV1Capabilities in lm_studio_v1.go) sets it to exactly ["embedding"] for
// embedding models and never mixes that entry with others, so a single "embedding" entry
// unambiguously means "embedding model" regardless of what Type says.
//
// The v0 sync path never populates Capabilities (see
// TestConvertToInternalModelV0PathLeavesV1MetadataEmpty), so on a v0-only host every model
// falls into that empty case. There, Type is whatever LM Studio v0 emitted verbatim, and
// its vocabulary is believed to be "llm" / "vlm" / "embeddings" (plural) - v0 predates the
// "embedding" (singular) spelling v1 introduced. Matching both spellings keeps this correct
// under either belief without needing to distinguish v0 from v1 explicitly.
func isEmbeddingModel(model entity.Model) bool {
	if len(model.Capabilities) > 0 {
		return len(model.Capabilities) == 1 && model.Capabilities[0] == TYPE_EMBEDDING
	}

	return model.Type == TYPE_EMBEDDING || model.Type == TYPE_EMBEDDING_PLURAL
}
