package notary

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	notationlog "github.com/notaryproject/notation-go/log"
)

type notaryLoggerAdapter struct {
	logger logr.Logger
}

func NotaryLoggerAdapter(logger logr.Logger) notationlog.Logger {
	return &notaryLoggerAdapter{
		logger: logger.V(4),
	}
}

func (nla *notaryLoggerAdapter) Debug(args ...interface{}) {
	nla.info(0, args...)
}

func (nla *notaryLoggerAdapter) Debugf(format string, args ...interface{}) {
	nla.infof(0, format, args...)
}

func (nla *notaryLoggerAdapter) Debugln(args ...interface{}) {
	nla.infoln(0, args...)
}

func (nla *notaryLoggerAdapter) Info(args ...interface{}) {
	nla.info(1, args...)
}

func (nla *notaryLoggerAdapter) Infof(format string, args ...interface{}) {
	nla.infof(1, format, args...)
}

func (nla *notaryLoggerAdapter) Infoln(args ...interface{}) {
	nla.infoln(1, args...)
}

func (nla *notaryLoggerAdapter) Warn(args ...interface{}) {
	nla.info(2, args...)
}

func (nla *notaryLoggerAdapter) Warnf(format string, args ...interface{}) {
	nla.infof(2, format, args...)
}

func (nla *notaryLoggerAdapter) Warnln(args ...interface{}) {
	nla.infoln(2, args...)
}

func (nla *notaryLoggerAdapter) Error(args ...interface{}) {
	nla.logger.Error(errors.New(fmt.Sprint(args...)), "")
}

func (nla *notaryLoggerAdapter) Errorf(format string, args ...interface{}) {
	nla.logger.Error(fmt.Errorf(format, args...), "")
}

func (nla *notaryLoggerAdapter) Errorln(args ...interface{}) {
	nla.logger.Error(errors.New(fmt.Sprintln(args...)), "")
}

func (nla *notaryLoggerAdapter) info(level int, args ...interface{}) {
	nla.log(level, fmt.Sprint(args...))
}

func (nla *notaryLoggerAdapter) infof(level int, format string, args ...interface{}) {
	nla.log(level, fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) infoln(level int, args ...interface{}) {
	nla.log(level, fmt.Sprintln(args...))
}

func (nla *notaryLoggerAdapter) log(level int, message string) {
	logger := nla.logger
	if level > 0 {
		logger = logger.V(level)
	}
	logger.Info(message)
}
