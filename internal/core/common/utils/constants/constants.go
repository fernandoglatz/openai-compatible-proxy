package constants

type ContextKey string

const (
	TRACE_MAP ContextKey = "TRACE-MAP"

	LOGGING_LEVEL = "LOGGING_LEVEL"
	PROFILE       = "PROFILE"
	DEV_PROFILE   = "dev"

	ID         string = "id"
	REQUEST_ID string = "REQUEST-ID"

	AUTHORIZATION string = "Authorization"
	BEARER_PREFIX string = "Bearer "
	CONTENT_TYPE  string = "Content-Type"

	ZERO = 0
	ONE  = 1
	TEN  = 10
)
