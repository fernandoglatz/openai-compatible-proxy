package service

import (
	"context"
	"net/http"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"

	"github.com/gin-gonic/gin"
)

type ILMStudioService interface {
	FetchAndSaveModels(ctx context.Context) *exceptions.WrappedError
	ProxyRequestStreaming(ctx context.Context, ginCtx *gin.Context, method string, path string, requestBody []byte, headers http.Header) error
}
