package controller

import (
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/port/service"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

type LMStudioProxyController struct {
	lmStudioService service.ILMStudioService
}

func NewLMStudioProxyController(lmStudioService service.ILMStudioService) *LMStudioProxyController {
	return &LMStudioProxyController{
		lmStudioService: lmStudioService,
	}
}

func (controller *LMStudioProxyController) ProxyRequest(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)

	// Get the full path from the request (e.g., /v1/models, /v1/chat/completions)
	path := ginCtx.Request.URL.Path

	// Skip proxying for /v1/models - it's handled by OpenAIController
	if path == "/v1/models" || path == "/v1/models/" {
		ginCtx.Next()
		return
	}

	log.Info(ctx).Msg("Forwarding /v1 request to LM Studio")

	// Read the request body
	body, err := io.ReadAll(ginCtx.Request.Body)
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to read request body: %v", err))
		ginCtx.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Forward to LM Studio using the service's proxy functionality
	responseBody, statusCode, err := controller.lmStudioService.ProxyRequest(ctx, ginCtx.Request.Method, path, body, ginCtx.Request.Header)
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to forward request to LM Studio: %v", err))
		ginCtx.JSON(statusCode, gin.H{"error": "failed to forward request to LM Studio"})
		return
	}

	// Return the response from LM Studio
	ginCtx.Data(statusCode, gin.MIMEJSON, responseBody)
}
