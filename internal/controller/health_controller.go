package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthController struct {
}

func NewHealthController() *HealthController {
	return &HealthController{}
}

// @Tags	health
// @Summary	Get health
// @Produce	text/plain
// @Success	200	{object}	string
// @Failure	400	{object}	string
// @Failure	500	{object}	string
// @Router	/health [get]
func (healthController *HealthController) Health(ginCtx *gin.Context) {
	ginCtx.String(http.StatusOK, "OK")
}
