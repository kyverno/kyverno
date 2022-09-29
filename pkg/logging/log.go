package logging

import (
	"errors"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// JSONFormat represents JSON logging mode.
	JSONFormat = "json"
	// TextFormat represents text logging mode.
	// Default logging mode is TextFormat.
	TextFormat = "text"
)

// Setup configures the logger with the supplied log format.
// It returns an error if the JSON logger could not be initialized or passed logFormat is not recognized.
func Setup(logFormat string) error {
	klog.InitFlags(nil)

	switch logFormat {
	case TextFormat:
		// in text mode we use FormatSerialize format
		log.SetLogger(klogr.New())
		return nil
	case JSONFormat:
		zapLog, err := zap.NewProduction()
		if err != nil {
			return errors.New("JSON logger could not be initialized")
		}
		klog.SetLogger(zapr.NewLogger(zapLog))

		// in json mode we use FormatKlog format
		log.SetLogger(klog.NewKlogr())
		return nil
	}
	return errors.New("log format not recognized, pass `text` for text mode or `json` to enable JSON logging")
}

// GlobalLogger returns a logr.Logger as configured in main.
func GlobalLogger() logr.Logger {
	return log.Log
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
	GlobalLogger().Info(msg, keysAndValues...)
}

// Error logs an error, with the given message and key/value pairs.
func Error(err error, msg string, keysAndValues ...interface{}) {
	GlobalLogger().Error(err, msg, keysAndValues...)
}
