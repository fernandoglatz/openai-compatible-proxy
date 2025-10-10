package server

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/controller"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/router"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"fmt"

	"github.com/gin-gonic/gin"
)

func Setup(ctx context.Context) error {
	log.Info(ctx).Msg("Starting web server")

	serverConfig := config.ApplicationConfig.Server
	contextPath := serverConfig.ContextPath
	listening := serverConfig.Listening

	if log.IsLevelEnabled(log.DEBUG) {
		gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
			formattedMessage := fmt.Sprintf("Routing %v\t%v\t-->\t%v\t(%v handlers)", httpMethod, absolutePath, handlerName, nuHandlers)

			log.Debug(ctx).Msg(formattedMessage)
		}

	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.RedirectTrailingSlash = false
	engine.Use(
		controller.TraceMiddleware(),
		controller.LoggingMiddleware(),
		controller.RecoveryMiddleware(ctx),
	)

	router.Setup(ctx, engine)

	log.Info(ctx).Msg("Web server listening on " + listening + contextPath)
	return engine.Run(listening)
}
