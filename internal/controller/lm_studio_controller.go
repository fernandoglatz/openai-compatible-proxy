package controller

import (
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/response"
	"fernandoglatz/openai-compatible-proxy/internal/core/port/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type LMStudioController struct {
	modelService    service.IModelService
	lmStudioService service.ILMStudioService
}

func NewLMStudioController(modelService service.IModelService, lmStudioService service.ILMStudioService) *LMStudioController {
	return &LMStudioController{
		modelService:    modelService,
		lmStudioService: lmStudioService,
	}
}

// @Tags	lm-studio
// @Summary	List available models
// @Produce	json
// @Success	200	{object}	response.LMStudioListResponse
// @Router	/api/v0/models [get]
func (controller *LMStudioController) ListModels(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)

	// Try to fetch and save models from LM Studio, but continue even if it fails
	err := controller.lmStudioService.FetchAndSaveModels(ctx)
	if err != nil {
		log.Warn(ctx).Msg("LM Studio API not available or error fetching models, returning only database models")
	}

	log.Info(ctx).Msg("Listing LM Studio models")

	models, err := controller.modelService.GetAll(ctx)
	if err != nil {
		log.Error(ctx).Msg("Failed to get models")
		ginCtx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch models"})
		return
	}

	response := controller.convertToLMStudioListResponse(models)

	ginCtx.JSON(http.StatusOK, response)
}

// @Tags	lm-studio
// @Summary	Get details of a specific model by ID
// @Produce	json
// @Param	model	path	string	true	"Model ID"
// @Success	200	{object}	response.LMStudioModel
// @Router	/api/v0/models/{model} [get]
func (controller *LMStudioController) GetModel(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)
	modelID := ginCtx.Param("model")

	log.Info(ctx).Msg("Getting LM Studio model details for: " + modelID)

	model, err := controller.modelService.GetByName(ctx, modelID)
	if err != nil {
		log.Error(ctx).Msg("Model not found: " + modelID)
		ginCtx.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}

	response := controller.convertToLMStudioModel(model)

	ginCtx.JSON(http.StatusOK, response)
}

func (controller *LMStudioController) convertToLMStudioListResponse(models []entity.Model) response.LMStudioListResponse {
	lmStudioModels := make([]response.LMStudioModel, 0)

	for _, model := range models {
		lmStudioModels = append(lmStudioModels, controller.convertToLMStudioModel(model))
	}

	return response.LMStudioListResponse{
		Object: "list",
		Data:   lmStudioModels,
	}
}

func (controller *LMStudioController) convertToLMStudioModel(model entity.Model) response.LMStudioModel {
	return response.LMStudioModel{
		ID:                model.Name,
		Object:            model.Object,
		Type:              model.Type,
		Publisher:         model.Publisher,
		Arch:              model.Arch,
		CompatibilityType: model.CompatibilityType,
		Quantization:      model.Quantization,
		State:             model.State,
		MaxContextLength:  model.MaxContextLength,
	}
}
