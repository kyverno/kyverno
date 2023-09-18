package logging

import (
	"fmt"

	"github.com/go-logr/logr"
)

type DebugLogger interface {
	// Debug logs a debug level message.
	Debug(args ...interface{})

	// Debugf logs a debug level message with format.
	Debugf(format string, args ...interface{})

	// Debugln logs a debug level message. Spaces are always added between
	// operands.
	Debugln(args ...interface{})

	// Info logs an info level message.
	Info(args ...interface{})

	// Infof logs an info level message with format.
	Infof(format string, args ...interface{})

	// Infoln logs an info level message. Spaces are always added between
	// operands.
	Infoln(args ...interface{})

	// Warn logs a warn level message.
	Warn(args ...interface{})

	// Warnf logs a warn level message with format.
	Warnf(format string, args ...interface{})

	// Warnln logs a warn level message. Spaces are always added between
	// operands.
	Warnln(args ...interface{})

	// Error logs an error level message.
	Error(args ...interface{})

	// Errorf logs an error level message with format.
	Errorf(format string, args ...interface{})

	// Errorln logs an error level message. Spaces are always added between
	// operands.
	Errorln(args ...interface{})
}

func NewDebugLogger(logger logr.Logger) DebugLogger {
	return &debugLogger{
		logger: logger,
	}
}

type debugLogger struct {
	logger logr.Logger
}

func (dl *debugLogger) Debug(args ...interface{}) {
	dl.info(args...)
}

func (dl *debugLogger) Debugf(format string, args ...interface{}) {
	dl.logger.Info(fmt.Sprintf(format, args...))
}

func (dl *debugLogger) Debugln(args ...interface{}) {
	dl.info(args...)
}

func (dl *debugLogger) Info(args ...interface{}) {
	dl.info(args...)
}

func (dl *debugLogger) Infof(format string, args ...interface{}) {
	dl.logger.Info(fmt.Sprintf(format, args...))
}

func (dl *debugLogger) Infoln(args ...interface{}) {
	dl.info(args...)
}

func (dl *debugLogger) Warn(args ...interface{}) {
	dl.info(args...)
}

func (dl *debugLogger) Warnf(format string, args ...interface{}) {
	dl.logger.Info(fmt.Sprintf(format, args...))
}

func (dl *debugLogger) Warnln(args ...interface{}) {
	dl.info(args...)
}

func (dl *debugLogger) Error(args ...interface{}) {
	dl.info(args...)
}

func (dl *debugLogger) Errorf(format string, args ...interface{}) {
	dl.logger.Info(fmt.Sprintf(format, args...))
}

func (dl *debugLogger) Errorln(args ...interface{}) {
	dl.info(args...)
}

func (dl *debugLogger) info(args ...interface{}) {
	if len(args) == 0 {
		return
	}

	msg, ok := args[0].(string)
	if !ok {
		return
	}

	if len(args) > 1 {
		dl.logger.Info(msg, args[1:]...)
	} else if len(args) == 1 {
		dl.logger.Info(msg)
	}
}
