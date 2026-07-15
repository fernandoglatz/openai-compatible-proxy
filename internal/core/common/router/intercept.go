package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// interceptGet routes a GET request whose path ends with suffix to handler, letting
// everything else fall through to the group's catch-all proxy.
//
// gin's router panics when a static route and a catch-all share a path segment in the
// same group, so a group that proxies /* cannot also register a real route for one path.
// Intercepting in middleware is the workaround.
func interceptGet(suffix string, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		if ginCtx.Request.Method == http.MethodGet && strings.HasSuffix(ginCtx.Request.URL.Path, suffix) {
			handler(ginCtx)
			ginCtx.Abort()
			return
		}

		ginCtx.Next()
	}
}
