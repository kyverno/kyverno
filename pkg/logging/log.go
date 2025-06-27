package logging

import (
	"context"
	"errors"
	"flag"
	"io"
	stdlog "log"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// JSONFormat represents JSON logging mode.
	JSONFormat = "json"
	// TextFormat represents text logging mode.
	// Default logging mode is TextFormat.
	TextFormat = "text"
	// LogLevelController is the log level to use for controllers plumbing.
	LogLevelController = 1
	// LogLevelClient is the log level to use for clients.
	LogLevelClient = 1
	// time formats
	DefaultTime = "default"
	ISO8601     = "iso8601"
	RFC3339     = "rfc3339"
	MILLIS      = "millis"
	NANOS       = "nanos"
	EPOCH       = "epoch"
	RFC3339NANO = "rfc3339nano"
)

// Initially, globalLog comes from controller-runtime/log with logger created earlier by controller-runtime.
// When logging.Setup is called, globalLog is switched to the real logger.
// Call depth of all loggers created before logging.Setup will not work, including package level loggers as they are created before main.
// All loggers created after logging.Setup won't be subject to the call depth limitation and will work if the underlying sink supports it.

var globalLog = log.Log // returns a Null log sink if SetLogger is not called.

func InitFlags(flags *flag.FlagSet) {
	// clear flags initialized in static dependencies
	if flag.CommandLine.Lookup("log_dir") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}
	klog.InitFlags(flags)
}

// Setup configures the logger with the supplied log format.
// It returns an error if the JSON logger could not be initialized or passed logFormat is not recognized.
func Setup(logFormat string, loggingTimestampFormat string, level int, disableColor bool) error {
	zerologr.SetMaxV(level)

	var logger zerolog.Logger
	output := zerolog.ConsoleWriter{Out: os.Stderr, NoColor: disableColor}
	output.TimeFormat = resolveTimestampFormat(loggingTimestampFormat)

	switch logFormat {
	case TextFormat:
		logger = zerolog.New(output).With().Timestamp().Caller().Logger()
	case JSONFormat:
		logger = zerolog.New(output).With().Timestamp().Logger()
	default:
		return errors.New("log format not recognized, pass `text` for text mode or `json` to enable JSON logging")
	}

	globalLog = zerologr.New(&logger)
	klog.SetLogger(globalLog.WithName("klog"))
	log.SetLogger(globalLog)
	return nil
}

func resolveTimestampFormat(format string) string {
	switch format {
	case ISO8601:
		return time.RFC3339
	case RFC3339:
		return time.RFC3339
	case MILLIS:
		return time.StampMilli
	case NANOS:
		return time.StampNano
	case EPOCH:
		return time.UnixDate
	case RFC3339NANO:
		return time.RFC3339Nano
	case DefaultTime:
		return time.RFC3339
	default:
		return time.RFC3339
	}
}

// GlobalLogger returns a logr.Logger as configured in main.
func GlobalLogger() logr.Logger {
	return globalLog
}

// ControllerLogger returns a logr.Logger to be used by controllers.
func ControllerLogger(name string) logr.Logger {
	return globalLog.WithName(name).V(LogLevelController)
}

// ClientLogger returns a logr.Logger to be used by clients.
func ClientLogger(name string) logr.Logger {
	return globalLog.WithName(name).V(LogLevelClient)
}

// WithName returns a new logr.Logger instance with the specified name element added to the Logger's name.
func WithName(name string) logr.Logger {
	return GlobalLogger().WithName(name)
}

// WithValues returns a new logr.Logger instance with additional key/value pairs.
func WithValues(keysAndValues ...interface{}) logr.Logger {
	return GlobalLogger().WithValues(keysAndValues...)
}

// V returns a new logr.Logger instance for a specific verbosity level.
func V(level int) logr.Logger {
	return GlobalLogger().V(level)
}

// Info logs a non-error message with the given key/value pairs.
func Info(msg string, keysAndValues ...interface{}) {
	GlobalLogger().WithCallDepth(1).Info(msg, keysAndValues...)
}

// Error logs an error, with the given message and key/value pairs.
func Error(err error, msg string, keysAndValues ...interface{}) {
	GlobalLogger().WithCallDepth(1).Error(err, msg, keysAndValues...)
}

// FromContext returns a logger with predefined values from a context.Context.
func FromContext(ctx context.Context, keysAndValues ...interface{}) (logr.Logger, error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		return logger, err
	}
	return logger.WithValues(keysAndValues...), nil
}

// IntoContext takes a context and sets the logger as one of its values.
// Use FromContext function to retrieve the logger.
func IntoContext(ctx context.Context, log logr.Logger) context.Context {
	return logr.NewContext(ctx, log)
}

// IntoBackground calls IntoContext with the logger and a Background context.
func IntoBackground(log logr.Logger) context.Context {
	return IntoContext(context.Background(), log)
}

// IntoTODO calls IntoContext with the logger and a TODO context.
func IntoTODO(log logr.Logger) context.Context {
	return IntoContext(context.TODO(), log)
}

// Background calls IntoContext with the global logger and a Background context.
func Background() context.Context {
	return IntoContext(context.Background(), GlobalLogger())
}

// TODO calls IntoContext with the global logger and a TODO context.
func TODO() context.Context {
	return IntoContext(context.TODO(), GlobalLogger())
}

type writerAdapter struct {
	io.Writer
	logger logr.Logger
}

func (a *writerAdapter) Write(p []byte) (int, error) {
	a.logger.Info(strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
}

func StdLogger(logger logr.Logger, prefix string) *stdlog.Logger {
	return stdlog.New(&writerAdapter{logger: logger}, prefix, stdlog.LstdFlags)
}
