package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/dto"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
)

// LMStudioAPI represents the client for communicating with LM Studio
type LMStudioAPI struct {
	baseURL    string
	httpClient *http.Client
}

// NewLMStudioAPI creates a new instance of LMStudioAPI client
func NewLMStudioAPI() *LMStudioAPI {
	config := config.ApplicationConfig.LMStudio

	// Check if the URL is valid
	if config.URL == "" {
		log.Fatal(context.Background()).Msg("LM Studio URL configuration is empty")
	}

	return &LMStudioAPI{
		baseURL: config.URL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// GetBaseURL returns the base URL of the LM Studio API
func (client *LMStudioAPI) GetBaseURL() string {
	return client.baseURL
}

// GetHTTPClient returns the HTTP client used for requests
func (client *LMStudioAPI) GetHTTPClient() *http.Client {
	return client.httpClient
}

// GetModels fetches all models from the LM Studio API
func (client *LMStudioAPI) GetModels(ctx context.Context) ([]dto.LMStudioModel, error) {
	log.Info(ctx).Msg("Fetching models from LM Studio")

	url := fmt.Sprintf("%s/api/v0/models", client.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from LM Studio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio API returned status code: %d", resp.StatusCode)
	}

	var apiResponse dto.LMStudioResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode LM Studio response: %w", err)
	}

	log.Info(ctx).Msg("Successfully fetched models from LM Studio")
	return apiResponse.Data, nil
}
