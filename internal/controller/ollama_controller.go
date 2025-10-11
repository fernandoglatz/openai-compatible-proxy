package controller

import (
	"crypto/sha256"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/request"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/response"
	"fernandoglatz/openai-compatible-proxy/internal/core/port/service"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type OllamaController struct {
	modelService    service.IModelService
	lmStudioService service.ILMStudioService
}

func NewOllamaController(modelService service.IModelService, lmStudioService service.ILMStudioService) *OllamaController {
	return &OllamaController{
		modelService:    modelService,
		lmStudioService: lmStudioService,
	}
}

// @Tags	ollama
// @Summary	Get tags from models
// @Produce	json
// @Success	200	{object}	response.OllamaResponse
// @Router	/api/tags [get]
func (controller *OllamaController) GetTags(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)

	// Try to fetch and save models from LM Studio, but continue even if it fails
	err := controller.lmStudioService.FetchAndSaveModels(ctx)
	if err != nil {
		log.Warn(ctx).Msg("LM Studio API not available or error fetching models, returning only database models")
	}

	log.Info(ctx).Msg("Getting Ollama tags")

	models, err := controller.modelService.GetAll(ctx)
	if err != nil {
		log.Error(ctx).Msg("Failed to get models")
		ginCtx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch models"})
		return
	}

	response := controller.convertToOllamaResponse(models)

	ginCtx.JSON(http.StatusOK, response)
}

// @Tags	ollama
// @Summary	Get version information
// @Produce	json
// @Success	200	{object}	response.OllamaVersionResponse
// @Router	/api/version [get]
func (controller *OllamaController) GetVersion(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)
	log.Info(ctx).Msg("Getting version information")

	ginCtx.JSON(http.StatusOK, response.OllamaVersionResponse{
		Version: "0.12.2",
	})
}

// @Tags	ollama
// @Summary	Get details of a specific model by name
// @Accept	json
// @Produce	json
// @Param	model	body	request.OllamaShowRequest	true	"Model name"
// @Success	200	{object}	response.OllamaModel
// @Router	/api/show [post]
func (controller *OllamaController) Show(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)
	log.Info(ctx).Msg("Getting Ollama model details")

	var request request.OllamaShowRequest
	if err := ginCtx.ShouldBindJSON(&request); err != nil {
		log.Error(ctx).Msg("Failed to parse request body")
		ginCtx.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse request body"})
		return
	}

	model, err := controller.modelService.GetByName(ctx, request.Model)
	if err != nil {
		log.Error(ctx).Msg("Failed to get model")
		ginCtx.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}

	response := controller.convertToOllamaResponse([]entity.Model{model})

	if len(response.Models) > 0 {
		ginCtx.JSON(http.StatusOK, response.Models[0])
	} else {
		ginCtx.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
	}
}

func (controller *OllamaController) convertToOllamaResponse(models []entity.Model) response.OllamaResponse {
	var ollamaModels []response.OllamaModel

	for _, model := range models {
		if model.Type == "llm" || model.Type == "vlm" {
			modifiedAt := model.UpdatedAt.Format(time.RFC3339)

			family := model.Name
			if idx := strings.Index(model.Name, "/"); idx != -1 {
				family = model.Name[:idx]
			}

			paramSize := extractParameterSize(model.Name)

			details := response.Details{
				Format:            model.Type,
				Family:            family,
				Families:          []string{family},
				ParameterSize:     paramSize,
				QuantizationLevel: model.Quantization,
			}

			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(model.Name)))

			capabilities := []string{"completion", "chat", "tools"}
			if model.Type == "vlm" {
				capabilities = append(capabilities, "vision")
			}

			ollamaModels = append(ollamaModels, response.OllamaModel{
				Name:         model.Name,
				Model:        model.Name,
				ModifiedAt:   modifiedAt,
				Size:         0,
				Digest:       digest,
				Details:      details,
				Capabilities: capabilities,
				ModelInfo:    map[string]string{"general.architecture": model.Arch},
			})
		}
	}

	return response.OllamaResponse{
		Models: ollamaModels,
	}
}

func extractParameterSize(name string) string {
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[bB]`)
	matches := re.FindAllStringSubmatch(name, -1)
	if len(matches) > 0 {
		return matches[0][1] + "B"
	}

	re = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[Mm]`)
	matches = re.FindAllStringSubmatch(name, -1)
	if len(matches) > 0 {
		return matches[0][1] + "M"
	}

	if strings.Contains(name, "8b") {
		return "8B"
	} else if strings.Contains(name, "13b") {
		return "13B"
	} else if strings.Contains(name, "30b") {
		return "30B"
	} else if strings.Contains(name, "70b") {
		return "70B"
	} else if strings.Contains(name, "405b") {
		return "405B"
	} else if strings.Contains(name, "268.10M") {
		return "268.10M"
	}

	return ""
}
