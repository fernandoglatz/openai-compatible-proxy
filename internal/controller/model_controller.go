package controller

import (
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/port/service"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ModelController struct {
	service         service.IModelService
	lmStudioService service.ILMStudioService
}

func NewModelController(service service.IModelService, lmStudioService service.ILMStudioService) *ModelController {
	return &ModelController{
		service:         service,
		lmStudioService: lmStudioService,
	}
}

// @Tags	model
// @Summary	Get models
// @Produce	json
// @Success	200	{array}		entity.Model
// @Failure	400	{object}	response.Response
// @Failure	500	{object}	response.Response
// @Router	/model [get]
func (controller *ModelController) Get(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)
	log.Info(ctx).Msg("Getting models")

	models, err := controller.service.GetAll(ctx)
	if err != nil {
		HandleError(ctx, ginCtx, err)
		return
	}

	ginCtx.JSON(http.StatusOK, models)
}

// @Tags	model
// @Summary	Get model
// @Param	id		path	string  true "id"
// @Produce	json
// @Success	200	{object}	entity.Model
// @Failure	400	{object}	response.Response
// @Failure	500	{object}	response.Response
// @Router	/model/{id} [get]
func (controller *ModelController) GetId(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)
	id := ginCtx.Param("id")

	log.Info(ctx).Msg(fmt.Sprintf("Getting model %s", id))

	model, err := controller.service.Get(ctx, id)
	if err != nil {
		HandleError(ctx, ginCtx, err)
		return
	}

	ginCtx.JSON(http.StatusOK, model)
}

// @Tags	model
// @Summary	Fetch and save models from LM Studio
// @Produce	json
// @Success	200	{array}		entity.Model
// @Failure	400	{object}	response.Response
// @Failure	500	{object}	response.Response
// @Router	/model/lm-studio [get]
func (controller *ModelController) GetLMStudioModels(ginCtx *gin.Context) {
	ctx := GetContext(ginCtx)
	log.Info(ctx).Msg("Fetching and saving models from LM Studio")

	err := controller.lmStudioService.FetchAndSaveModels(ctx)
	if err != nil {
		HandleError(ctx, ginCtx, err)
		return
	}

	// After fetching and saving, return all models
	models, err := controller.service.GetAll(ctx)
	if err != nil {
		HandleError(ctx, ginCtx, err)
		return
	}

	ginCtx.JSON(http.StatusOK, models)
}
