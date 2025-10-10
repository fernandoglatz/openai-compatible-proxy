package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/dto"
	"fernandoglatz/openai-compatible-proxy/internal/core/port/service"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/api"
)

// LMStudioService handles interaction with LM Studio and model persistence
type LMStudioService struct {
	lmStudioAPI  *api.LMStudioAPI
	modelService service.IModelService
}

// NewLMStudioService creates a new instance of LMStudioService
func NewLMStudioService(lmStudioAPI *api.LMStudioAPI, modelService service.IModelService) *LMStudioService {
	return &LMStudioService{
		lmStudioAPI:  lmStudioAPI,
		modelService: modelService,
	}
}

// FetchAndSaveModels fetches models from LM Studio and saves them to the database
func (service *LMStudioService) FetchAndSaveModels(ctx context.Context) *exceptions.WrappedError {
	log.Info(ctx).Msg("Starting to fetch models from LM Studio")

	models, err := service.lmStudioAPI.GetModels(ctx)
	if err != nil {
		return &exceptions.WrappedError{
			Message:   "failed to get models from LM Studio",
			BaseError: exceptions.GenericError,
			Error:     err,
		}
	}

	log.Info(ctx).Msg(fmt.Sprintf("Fetched %d models from LM Studio. Saving to database", len(models)))

	// Get all existing models from the database
	allExistingModels, getAllErr := service.modelService.GetAll(ctx)
	if getAllErr != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to get existing models: %v", getAllErr))
		return getAllErr
	}

	// Convert dto.LMStudioModel to internal Model entity
	internalModels := make([]entity.Model, len(models))
	for i, lmModel := range models {
		internalModels[i] = service.convertToInternalModel(lmModel)
	}

	// Create a map of LM Studio model IDs for quick lookup
	lmStudioModelIDs := make(map[string]bool)
	for _, model := range internalModels {
		lmStudioModelIDs[model.Name] = true
	}

	// Save each model to database
	for _, model := range internalModels {
		err := service.saveModel(ctx, model)
		if err != nil {
			log.Error(ctx).Msg(fmt.Sprintf("Failed to save model %s: %v", model.ID, err))
			// Continue with other models instead of failing completely
			continue
		}
	}

	log.Info(ctx).Msg(fmt.Sprintf("Successfully saved %d models to database", len(internalModels)))

	// Remove models from database that no longer exist in LM Studio
	for _, existingModel := range allExistingModels {
		if !lmStudioModelIDs[existingModel.Name] {
			log.Info(ctx).Msg(fmt.Sprintf("Removing model %s from database as it no longer exists in LM Studio", existingModel.Name))
			err := service.modelService.Remove(ctx, existingModel)
			if err != nil {
				log.Error(ctx).Msg(fmt.Sprintf("Failed to remove model %s: %v", existingModel.Name, err))
				// Continue with other models instead of failing completely
				continue
			}
		}
	}

	log.Info(ctx).Msg("Finished synchronizing database with LM Studio models")
	return nil
}

// convertToInternalModel converts LMStudioModel to internal Model entity
func (service *LMStudioService) convertToInternalModel(lmModel dto.LMStudioModel) entity.Model {
	model := entity.Model{
		Name:              lmModel.ID,
		Object:            lmModel.Object,
		Type:              lmModel.Type,
		Publisher:         lmModel.Publisher,
		Arch:              lmModel.Arch,
		CompatibilityType: lmModel.CompatibilityType,
		Quantization:      lmModel.Quantization,
		State:             lmModel.State,
		MaxContextLength:  lmModel.MaxContextLength,
	}

	return model
}

// saveModel saves a single model to the database
func (service *LMStudioService) saveModel(ctx context.Context, model entity.Model) error {
	// Check if model already exists by name
	existingModel, err := service.modelService.GetByName(ctx, model.Name)
	if err == nil {
		model.ID = existingModel.ID               // Preserve existing ID
		model.CreatedAt = existingModel.CreatedAt // Preserve original creation date
	}

	// Save the model to database
	err = service.modelService.Save(ctx, &model)
	if err != nil {
		return fmt.Errorf("failed to save model %s: %v", model.Name, err)
	}

	return nil
}

// ProxyRequest forwards a request to LM Studio API with appropriate headers and body
func (service *LMStudioService) ProxyRequest(ctx context.Context, method string, path string, requestBody []byte, headers http.Header) ([]byte, int, error) {
	log.Info(ctx).Msg(fmt.Sprintf("Proxying %s request to LM Studio at path: %s", method, path))

	// Construct URL - using the base URL from LMStudioAPI
	url := fmt.Sprintf("%s%s", service.lmStudioAPI.GetBaseURL(), path)
	log.Info(ctx).Msg(fmt.Sprintf("Forwarding request to: %s", url))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to create request: %v", err))
		return []byte(`{"error": "failed to create request"}`), http.StatusInternalServerError, err
	}

	// Copy headers from the original request
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Set content-type if not already set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Send request to LM Studio
	resp, err := service.lmStudioAPI.GetHTTPClient().Do(req)
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to send request to LM Studio: %v", err))
		return []byte(`{"error": "failed to send request to LM Studio"}`), http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to read response body: %v", err))
		return []byte(`{"error": "failed to read response"}`), http.StatusInternalServerError, err
	}

	// Return response body and status code
	return responseBody, resp.StatusCode, nil
}
