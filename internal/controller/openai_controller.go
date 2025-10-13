package controller

import (
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/response"
	"fernandoglatz/openai-compatible-proxy/internal/core/port/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type OpenAIController struct {
	modelService    service.IModelService
	lmStudioService service.ILMStudioService
}

func NewOpenAIController(modelService service.IModelService, lmStudioService service.ILMStudioService) *OpenAIController {
	return &OpenAIController{
		modelService:    modelService,
		lmStudioService: lmStudioService,
	}
}

// @Tags	openai
// @Summary	List available models
// @Produce	json
// @Success	200	{object}	response.OpenAIListResponse
// @Router	/v1/models [get]
func (controller *OpenAIController) ListModels(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)

	// Try to fetch and save models from LM Studio, but continue even if it fails
	err := controller.lmStudioService.FetchAndSaveModels(ctx)
	if err != nil {
		log.Warn(ctx).Msg("LM Studio API not available or error fetching models, returning only database models")
	}

	log.Info(ctx).Msg("Listing OpenAI models")

	models, err := controller.modelService.GetAll(ctx)
	if err != nil {
		log.Error(ctx).Msg("Failed to get models")
		ginCtx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch models"})
		return
	}

	response := controller.convertToOpenAIListResponse(models)

	ginCtx.JSON(http.StatusOK, response)
}

func (controller *OpenAIController) convertToOpenAIListResponse(models []entity.Model) response.OpenAIListResponse {
	openAIModels := make([]response.OpenAIModel, 0)

	for _, model := range models {
		openAIModels = append(openAIModels, controller.convertToOpenAIModel(model))
	}

	return response.OpenAIListResponse{
		Object: "list",
		Data:   openAIModels,
	}
}

func (controller *OpenAIController) convertToOpenAIModel(model entity.Model) response.OpenAIModel {
	return response.OpenAIModel{
		ID:      model.Name,
		Object:  "model",
		Created: model.CreatedAt.Unix(),
		OwnedBy: "library",
	}
}
