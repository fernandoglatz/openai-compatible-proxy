package service

import (
	"context"
	"net/http"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
)

type ILMStudioService interface {
	FetchAndSaveModels(ctx context.Context) *exceptions.WrappedError
	ProxyRequest(ctx context.Context, method string, path string, requestBody []byte, headers http.Header) ([]byte, int, error)
}
