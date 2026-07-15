package router

import (
	"context"
	_ "fernandoglatz/openai-compatible-proxy/docs"
	"fernandoglatz/openai-compatible-proxy/internal/controller"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/service"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/api"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/repository"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Setup(ctx context.Context, engine *gin.Engine) {
	log.Info(ctx).Msg("Configuring routes")

	contextPath := config.ApplicationConfig.Server.ContextPath
	router := engine.Group(contextPath)

	modelRepository := repository.NewModelRepository()
	modelService := service.NewModelService(modelRepository)
	lmStudioAPI := api.NewLMStudioAPI()
	lmStudioService := service.NewLMStudioService(lmStudioAPI, modelService)
	lmStudioProxyController := controller.NewLMStudioProxyController(lmStudioService)
	lmStudioController := controller.NewLMStudioController(modelService, lmStudioService)
	lmStudioV1Controller := controller.NewLMStudioV1Controller(modelService, lmStudioService)
	ollamaController := controller.NewOllamaController(modelService, lmStudioService)
	openAIController := controller.NewOpenAIController(modelService, lmStudioService)

	healthController := controller.NewHealthController()

	// OpenAI routes - authenticated, use middleware to handle /v1/models specifically before proxy
	routerV1 := router.Group("/v1")
	routerV1.Use(controller.AuthenticationMiddleware(ctx))
	routerV1.Use(func(c *gin.Context) {
		// Handle /v1/models specifically
		if c.Request.URL.Path == "/v1/models" && c.Request.Method == "GET" {
			openAIController.ListModels(c)
			c.Abort()
			return
		}
		c.Next()
	})
	// Proxy catches all /v1/* requests
	routerV1.Any("/*any", lmStudioProxyController.ProxyRequest)

	// Ollama API routes
	routerAPI := router.Group("/api")
	routerAPI.GET("/tags", ollamaController.GetTags)
	routerAPI.POST("/show", ollamaController.Show)
	routerAPI.GET("/version", ollamaController.GetVersion)

	// LM Studio API routes - use middleware to handle specific routes before proxy
	routerAPIV0 := routerAPI.Group("/v0")
	routerAPIV0.Use(func(c *gin.Context) {
		// Handle specific /api/v0 routes
		if c.Request.Method == "GET" {
			if c.Request.URL.Path == "/api/v0/models" {
				lmStudioController.ListModels(c)
				c.Abort()
				return
			}
			// Handle /api/v0/models/:model pattern
			if len(c.Request.URL.Path) > 15 && c.Request.URL.Path[:15] == "/api/v0/models/" {
				// Extract the model parameter
				c.Params = append(c.Params, gin.Param{Key: "model", Value: c.Request.URL.Path[15:]})
				lmStudioController.GetModel(c)
				c.Abort()
				return
			}
		}
		c.Next()
	})
	// Proxy catches all remaining /api/v0/* requests
	routerAPIV0.Any("/*any", lmStudioProxyController.ProxyRequest)

	// LM Studio native v1 API routes (LM Studio 0.4.0+). Authenticated as a whole group:
	// unlike v0, this namespace exposes mutating endpoints (load, unload, download).
	routerAPIV1 := routerAPI.Group("/v1")
	routerAPIV1.Use(controller.AuthenticationMiddleware(ctx))
	routerAPIV1.Use(interceptGet("/api/v1/models", lmStudioV1Controller.ListModels))
	// Proxy catches chat, models/load, models/unload, models/download, models/download/status
	routerAPIV1.Any("/*any", lmStudioProxyController.ProxyRequest)

	router.GET("/health", healthController.Health)
	router.GET("/swagger-ui/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.Info(ctx).Msg("Routes configured")
}
