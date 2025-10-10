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
	modelController := controller.NewModelController(modelService, lmStudioService)
	lmStudioProxyController := controller.NewLMStudioProxyController(lmStudioService)
	ollamaController := controller.NewOllamaController(modelService)

	healthController := controller.NewHealthController()

	routerModel := router.Group("/model")
	routerModel.GET("", modelController.Get)
	routerModel.GET(":id", modelController.GetId)
	routerModel.GET("/lm-studio", modelController.GetLMStudioModels)

	router.GET("/health", healthController.Health)
	router.GET("/swagger-ui/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// OpenAI proxy routes
	routerV1 := engine.Group("/v1")
	routerV1.Any("*any", lmStudioProxyController.ProxyRequest)

	// Ollama API routes
	routerAPI := engine.Group("/api")
	routerAPI.GET("/tags", ollamaController.GetTags)
	routerAPI.POST("/show", ollamaController.Show)
	routerAPI.GET("/version", ollamaController.GetVersion)

	// LM Studio proxy routes
	routerAPIV0 := routerAPI.Group("/v0")
	routerAPIV0.Any("*any", lmStudioProxyController.ProxyRequest)

	log.Info(ctx).Msg("Routes configured")
}
