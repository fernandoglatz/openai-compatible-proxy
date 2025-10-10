package constants

type ContextKey string

const (
	TRACE_MAP ContextKey = "TRACE-MAP"

	LOGGING_LEVEL = "LOGGING_LEVEL"
	PROFILE       = "PROFILE"
	DEV_PROFILE   = "dev"

	ID         string = "id"
	REQUEST_ID string = "REQUEST-ID"

	ZERO = 0
	ONE  = 1
	TEN  = 10
)
