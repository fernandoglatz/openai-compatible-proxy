package controller

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/response"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const FORMAT_TRACE_STR = "[%.3fms] HTTP %d %s %s %s"

// MAX_LOGGED_BODY_BYTES caps how much of a request body reaches the TRACE log.
const MAX_LOGGED_BODY_BYTES = 2048

func TraceMiddleware() gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		ctx := GetContext(ginCtx)
		requestId := uuid.New().String()

		traceMap := make(map[string]any)
		traceMap[constants.REQUEST_ID] = requestId

		ctx = context.WithValue(ctx, constants.TRACE_MAP, traceMap)
		ginCtx.Request = ginCtx.Request.WithContext(ctx)
		ginCtx.Next()
	}
}

// PathTraversalMiddleware rejects any request whose path contains dot-segments (e.g.
// "/api/v0/../v1/models/load", or its %2e%2e-encoded form once gin has decoded it).
// gin does not clean dot-segments before route matching, so without this check a request
// can be crafted to match an unauthenticated group (e.g. /api/v0) while its raw path
// string - forwarded verbatim to the upstream by the proxy controllers - actually targets
// an authenticated one (e.g. /api/v1). This is registered once, globally, so it covers
// every route group rather than relying on each group to defend itself.
func PathTraversalMiddleware() gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		if !isCleanRequestPath(ginCtx.Request.URL.Path) {
			ctx := GetContext(ginCtx)
			request := ginCtx.Request

			log.Warn(ctx).Msg("[" + request.Method + "] " + request.URL.Path + " - rejected, path contains dot-segments")

			ginCtx.AbortWithStatus(http.StatusBadRequest)
			return
		}

		ginCtx.Next()
	}
}

// isCleanRequestPath reports whether p is an absolute URL path with no dot-segments
// once every encoding layer is unwound. gin routes on the single-decoded URL.Path, but
// the proxy controllers re-parse the raw path string into the outgoing request, so a
// still-encoded "%2e%2e" that passes a single-layer check here reaches LM Studio intact
// and may be normalized there into a traversal. isCleanOnce checks one layer; the loop
// keeps unescaping and re-checking until nothing further decodes.
//
// Every exit rejects rather than accepts: a malformed escape or a path still decoding
// past the bound is treated as dirty. Legitimate paths decode fully within one or two
// layers, so only crafted input reaches those exits.
func isCleanRequestPath(p string) bool {
	if p == "" || !strings.HasPrefix(p, "/") {
		return false
	}

	for range 4 {
		if !isCleanOnce(p) {
			return false
		}

		decoded, err := url.PathUnescape(p)
		if err != nil {
			return false
		}

		if decoded == p {
			return true
		}

		p = decoded
	}

	return false
}

// isCleanOnce reports whether p, taken as-is (no further decoding), is an absolute URL
// path with no dot-segments ("." or "..") to clean away. It tolerates exactly one
// trailing slash - path.Clean strips it, so it is re-appended before comparing - because
// existing handlers (e.g. lm_studio_proxy_controller.go's /v1/models check) treat a
// legitimate trailing slash as equivalent to its absence. Anything path.Clean would
// otherwise change (dot-segments, duplicate slashes, etc.) is rejected.
func isCleanOnce(p string) bool {
	cleaned := path.Clean(p)

	if cleaned != "/" && strings.HasSuffix(p, "/") {
		cleaned += "/"
	}

	return cleaned == p
}

func LoggingMiddleware() gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		if !log.IsLevelEnabled(log.TRACE) {
			ginCtx.Next()
			return
		}

		ctx := GetContext(ginCtx)
		begin := time.Now()

		// Capture request body for logging
		var bodyBytes []byte
		if ginCtx.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(ginCtx.Request.Body)
			ginCtx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		contentType := ginCtx.Request.Header.Get(constants.CONTENT_TYPE)

		ginCtx.Next()

		elapsed := time.Since(begin)
		duration := float64(elapsed.Nanoseconds()) / 1e6
		reqUri := ginCtx.Request.RequestURI
		reqMethod := ginCtx.Request.Method
		statusCode := ginCtx.Writer.Status()
		clientIP := ginCtx.ClientIP()

		if !strings.Contains(reqUri, "/health") {
			formattedMessage := fmt.Sprintf(FORMAT_TRACE_STR, duration, statusCode, reqMethod, reqUri, clientIP)

			// Log request body if present
			if len(bodyBytes) > 0 {
				log.Trace(ctx).Msg(formattedMessage + " | Request Body: " + formatBodyForLog(bodyBytes, contentType))
			} else {
				log.Trace(ctx).Msg(formattedMessage)
			}
		}
	}
}

// formatBodyForLog keeps request bodies loggable when they carry images or audio:
// binary payloads are summarized rather than dumped, and long ones are truncated so
// a single base64 image cannot produce a multi-megabyte log line.
func formatBodyForLog(body []byte, contentType string) string {
	if isBinaryContentType(contentType) {
		return fmt.Sprintf("<%d bytes of %s omitted>", len(body), contentType)
	}

	if len(body) > MAX_LOGGED_BODY_BYTES {
		truncated := strings.ToValidUTF8(string(body[:MAX_LOGGED_BODY_BYTES]), "")
		return fmt.Sprintf("%s... <truncated, %d bytes total>", truncated, len(body))
	}

	return string(body)
}

func isBinaryContentType(contentType string) bool {
	if utils.IsEmptyStr(contentType) {
		return false
	}

	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[constants.ZERO]))

	return strings.HasPrefix(mediaType, "multipart/") ||
		strings.HasPrefix(mediaType, "audio/") ||
		strings.HasPrefix(mediaType, "image/") ||
		strings.HasPrefix(mediaType, "video/") ||
		mediaType == "application/octet-stream"
}

// AuthenticationMiddleware validates the Bearer token against the tokens configured
// in openai.api-keys. When no token is configured, authentication is disabled.
func AuthenticationMiddleware(ctx context.Context) gin.HandlerFunc {
	apiKeys := config.ApplicationConfig.OpenAI.APIKeys

	if len(apiKeys) == constants.ZERO {
		log.Warn(ctx).Msg("No [openai.api-keys] configured, authentication is disabled")

		return func(ginCtx *gin.Context) {
			ginCtx.Next()
		}
	}

	log.Info(ctx).Msg(fmt.Sprintf("Authentication enabled with %d configured token(s)", len(apiKeys)))

	return func(ginCtx *gin.Context) {
		token := extractBearerToken(ginCtx.Request.Header.Get(constants.AUTHORIZATION))

		if !isTokenAuthorized(token, apiKeys) {
			ctx := GetContext(ginCtx)
			request := ginCtx.Request

			log.Warn(ctx).Msg("[" + request.Method + "] " + request.URL.Path + " - rejected, missing or invalid token")

			ginCtx.AbortWithStatusJSON(http.StatusUnauthorized, response.OpenAIErrorResponse{
				Error: response.OpenAIError{
					Message: "Incorrect API key provided.",
					Type:    "invalid_request_error",
					Code:    "invalid_api_key",
				},
			})
			return
		}

		ginCtx.Next()
	}
}

func extractBearerToken(header string) string {
	prefix := constants.BEARER_PREFIX

	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}

	return strings.TrimSpace(header[len(prefix):])
}

// isTokenAuthorized compares the token against every configured key without
// short-circuiting, so the time taken does not reveal which key matched.
func isTokenAuthorized(token string, apiKeys []string) bool {
	if len(token) == constants.ZERO {
		return false
	}

	authorized := false

	for _, apiKey := range apiKeys {
		if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) == constants.ONE {
			authorized = true
		}
	}

	return authorized
}

func RecoveryMiddleware(ctx context.Context) gin.HandlerFunc {
	errorLogWriter := log.NewLogWritter(*log.Error(ctx))
	return gin.CustomRecoveryWithWriter(errorLogWriter, errorHandleRecovery)
}

func errorHandleRecovery(ginCtx *gin.Context, obj any) {
	ctx := GetContext(ginCtx)

	err, ok := obj.(error)
	if !ok {
		err = errors.New(exceptions.GenericError.Code)
	}

	errw := &exceptions.WrappedError{
		Error: err,
	}

	HandleError(ctx, ginCtx, errw)
}
