package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/dto"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
)

// LMStudioAPI represents the client for communicating with LM Studio
type LMStudioAPI struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewLMStudioAPI creates a new instance of LMStudioAPI client
func NewLMStudioAPI() *LMStudioAPI {
	config := config.ApplicationConfig.LMStudio

	// Check if the URL is valid
	if config.URL == "" {
		log.Fatal(context.Background()).Msg("LM Studio URL configuration is empty")
	}

	// Custom transport with reduced connection timeout. The short dial timeout is what
	// detects a sleeping host and triggers WOL; the overall deadline is Client.Timeout.
	// No ResponseHeaderTimeout: a non-streaming completion sends no headers until
	// generation finishes, which for vision or long outputs far exceeds any short value.
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   1 * time.Second, // Connection timeout
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &LMStudioAPI{
		baseURL: config.URL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
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

// applyAuthentication sets the configured LM Studio key on the request, if any
func (client *LMStudioAPI) applyAuthentication(req *http.Request) {
	if utils.IsNotEmptyStr(client.apiKey) {
		req.Header.Set(constants.AUTHORIZATION, constants.BEARER_PREFIX+client.apiKey)
	}
}

// errVersionUnsupported signals the endpoint is absent upstream (404/400), which is the
// only condition that justifies falling back to the legacy API. Connection errors must
// not fall back: a sleeping host fails both endpoints, so retrying would double the dial
// wait and log a misleading cause.
var errVersionUnsupported = errors.New("lm studio api version not supported")

// GetModels fetches all models from LM Studio, preferring the native v1 API and falling
// back to the legacy v0 API only when v1 is absent.
func (client *LMStudioAPI) GetModels(ctx context.Context) ([]dto.LMStudioModel, error) {
	models, err := client.getModelsV1(ctx)
	if err == nil {
		return models, nil
	}

	if !errors.Is(err, errVersionUnsupported) {
		return nil, err
	}

	log.Info(ctx).Msg("LM Studio /api/v1/models unavailable, falling back to /api/v0/models")

	return client.getModelsV0(ctx)
}

// getModelsV1 fetches models from the native v1 API (LM Studio 0.4.0+) and normalizes
// them into the shape shared with v0.
func (client *LMStudioAPI) getModelsV1(ctx context.Context) ([]dto.LMStudioModel, error) {
	log.Info(ctx).Msg("Fetching models from LM Studio /api/v1/models")

	url := fmt.Sprintf("%s/api/v1/models", client.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client.applyAuthentication(req)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from LM Studio v1: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("v1 returned status %d: %w", resp.StatusCode, errVersionUnsupported)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio v1 API returned status code: %d", resp.StatusCode)
	}

	var apiResponse dto.LMStudioV1Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode LM Studio v1 response: %w", err)
	}

	models := make([]dto.LMStudioModel, len(apiResponse.Models))
	for index, v1Model := range apiResponse.Models {
		models[index] = normalizeV1Model(v1Model)
	}

	log.Info(ctx).Msg("Successfully fetched models from LM Studio v1")

	return models, nil
}

// getModelsV0 fetches models from the legacy v0 API.
func (client *LMStudioAPI) getModelsV0(ctx context.Context) ([]dto.LMStudioModel, error) {
	log.Info(ctx).Msg("Fetching models from LM Studio /api/v0/models")

	url := fmt.Sprintf("%s/api/v0/models", client.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client.applyAuthentication(req)

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

// DoRequest executes an HTTP request to LM Studio
func (client *LMStudioAPI) DoRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	return client.httpClient.Do(req)
}
