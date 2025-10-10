package controller

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetContext(ginCtx *gin.Context) context.Context {
	return ginCtx.Request.Context()
}

func GetHeader(ctx *gin.Context, name string, required bool) (string, *exceptions.WrappedError) {
	header := ctx.Request.Header.Get(name)

	if utils.IsEmptyStr(header) && required {
		return header, &exceptions.WrappedError{
			Message:   "Header [" + name + "] não encontrado",
			BaseError: exceptions.HeaderNotFound,
		}
	}

	return header, nil
}

func HandleError(ctx context.Context, ginCtx *gin.Context, err *exceptions.WrappedError) {
	request := ginCtx.Request
	method := request.Method
	path := request.URL.Path

	code := err.GetCode()
	message := err.GetMessage()
	httpStatus := http.StatusBadRequest

	if err.Error != nil && len(err.BaseError.Code) == constants.ZERO {
		httpStatus = http.StatusInternalServerError
		code = http.StatusText(httpStatus)
		log.Error(ctx).Msg("[" + method + "] " + path + " - " + code + " - " + message)

	} else {
		log.Warn(ctx).Msg("[" + method + "] " + path + " - " + code + " - " + message)
	}

	if err.BaseError == exceptions.RecordNotFound {
		httpStatus = http.StatusNotFound
	}

	ginCtx.JSON(httpStatus, response.Response{
		Code:    code,
		Message: message,
	})
}
