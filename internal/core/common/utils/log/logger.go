package log

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config/format"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var currentLevel = TRACE
var currentFormat = format.TEXT

type LoggerEvent struct {
	traceMap map[string]any
	event    *zerolog.Event
	caller   string
	level    Level
}

type Level int

const (
	TRACE Level = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

const (
	DEV_PROFILE          = "dev"
	TIMESTAMP_LOG_FORMAT = "2006-01-02T15:04:05.999Z07:00"

	DEFAULT_CALLER_LEVEL = 2
)

func SetupLogger(profile string) {
	loggingLevel := os.Getenv(constants.LOGGING_LEVEL)
	setCurrentLevel(loggingLevel)

	if DEV_PROFILE == profile {
		setLoggerText(true)
	} else {
		setLoggerJson()
	}
}

func ReconfigureLogger(ctx context.Context, configFormat format.Format, level string, colored bool) {
	Info(ctx).Msg("Reconfiguring logger for level: " + strings.ToUpper(level))
	setCurrentLevel(level)

	if configFormat == format.TEXT {
		setLoggerText(colored)
	} else {
		setLoggerJson()
	}
}

func setCurrentLevel(level string) {
	switch strings.ToUpper(level) {
	case "FATAL":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
		currentLevel = FATAL
	case "ERROR":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		currentLevel = ERROR
	case "WARN":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
		currentLevel = WARN
	case "INFO":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		currentLevel = INFO
	case "DEBUG":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		currentLevel = DEBUG
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		currentLevel = TRACE
	}
}

func setLoggerJson() {
	currentFormat = format.JSON
	zerolog.TimeFieldFormat = TIMESTAMP_LOG_FORMAT
	log.Logger = zerolog.New(os.Stdout)
}

func setLoggerText(colored bool) {
	currentFormat = format.TEXT
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: TIMESTAMP_LOG_FORMAT,
		NoColor:    !colored,
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

func IsLevelEnabled(level Level) bool {
	return level >= currentLevel
}

func (loggerEvent *LoggerEvent) PutTraceMap(key string, value any) *LoggerEvent {
	loggerEvent.traceMap[key] = value
	return loggerEvent
}

func (loggerEvent *LoggerEvent) Caller(caller string) *LoggerEvent {
	loggerEvent.caller = caller
	return loggerEvent
}

func (loggerEvent *LoggerEvent) Wrap(err exceptions.WrappedError) {
	if IsLevelEnabled(loggerEvent.level) {
		message := err.GetMessage()
		loggerEvent.Msg(message)
	}
}

func (loggerEvent *LoggerEvent) Msg(msg string) {
	if IsLevelEnabled(loggerEvent.level) {
		now := time.Now()
		traceMap := loggerEvent.traceMap
		event := loggerEvent.event
		caller := loggerEvent.caller

		if format.JSON == currentFormat {
			event.Time("@timestamp", now)
			event.Str("logName", "openai-compatible-proxy")
			event.Str("logIndex", "clarofree")
		}

		if traceMap != nil {
			if format.TEXT == currentFormat {
				for key, value := range traceMap {
					event = event.Any(key, value)
				}
			} else {
				event = event.Interface("trace", traceMap)
			}
		}

		if caller != "" {
			event = event.Str("caller", caller)
		}

		event.Msg(msg)
	}
}

func Trace(ctx context.Context) *LoggerEvent {
	return CreateLoggerEvent(ctx, log.Trace(), TRACE)
}

func Debug(ctx context.Context) *LoggerEvent {
	return CreateLoggerEvent(ctx, log.Debug(), DEBUG)
}

func Info(ctx context.Context) *LoggerEvent {
	return CreateLoggerEvent(ctx, log.Info(), INFO)
}

func Warn(ctx context.Context) *LoggerEvent {
	return CreateLoggerEvent(ctx, log.Warn(), WARN)
}

func Error(ctx context.Context) *LoggerEvent {
	return CreateLoggerEvent(ctx, log.Error(), ERROR)
}

func Fatal(ctx context.Context) *LoggerEvent {
	return CreateLoggerEvent(ctx, log.Fatal(), FATAL)
}

func CreateLoggerEvent(ctx context.Context, event *zerolog.Event, level Level) *LoggerEvent {
	loggerEvent := &LoggerEvent{
		event: event,
		level: level,
	}

	traceObj := ctx.Value(constants.TRACE_MAP)
	if traceObj != nil {
		loggerEvent.traceMap = traceObj.(map[string]any)
	}

	_, file, no, ok := runtime.Caller(DEFAULT_CALLER_LEVEL)
	if ok {
		file = filepath.Base(file)
		loggerEvent.caller = file + ":" + strconv.Itoa(no)
	}

	return loggerEvent
}

type LogWritter struct {
	event LoggerEvent
}

func (logWritter *LogWritter) Write(data []byte) (n int, err error) {
	logWritter.event.Msg(string(data))
	return len(data), nil
}

func NewLogWritter(event LoggerEvent) *LogWritter {
	return &LogWritter{
		event: event,
	}
}
