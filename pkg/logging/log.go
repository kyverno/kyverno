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

type Logger = logr.Logger

const (
	JSONFormat = "json"
	TextFormat = "text"
)

//Confugures the logger with the supplied log format
func Setup(logFormat string) error {
	var setupErr error

	switch logFormat {
	case TextFormat:
		// in text mode we use FormatSerialize format
		log.SetLogger(klogr.New())
	case JSONFormat:
		zapLog, err := zap.NewProduction()
		if err != nil {
			setupErr = errors.New("JSON logger could not be initialized")
		}
		klog.SetLogger(zapr.NewLogger(zapLog))

		// in json mode we use FormatKlog format
		log.SetLogger(klog.NewKlogr())
		return setupErr
	default:
		setupErr = errors.New("log format not recognized, pass `text` for text mode or `json` to enable JSON logging")
	}
	return setupErr
}

//Returns the global logger as configured in main
func GlobalLogger() Logger {
	return log.Log
}

//Return a new Logger instance with the specified name element added to the Logger's name
func WithName(name string) Logger {
	return GlobalLogger().WithName(name)
}

//WithValues returns a new Logger instance with additional key/value pairs
func WithValues(keysAndValues ...interface{}) Logger {
	return GlobalLogger().WithValues(keysAndValues...)
}

//V returns a new Logger instance for a specific verbosity level
func V(level int) Logger {
	return GlobalLogger().V(level)
}
