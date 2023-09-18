package notary

import (
	"fmt"

	"github.com/go-logr/logr"
	notationlog "github.com/notaryproject/notation-go/log"
)

func NotaryLoggerAdapter(logger logr.Logger) notationlog.Logger {
	return &notaryLoggerAdapter{
		logger: logger,
	}
}

type notaryLoggerAdapter struct {
	logger logr.Logger
}

func (nla *notaryLoggerAdapter) Debug(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Debugf(format string, args ...interface{}) {
	nla.logger.Info(fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) Debugln(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Info(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Infof(format string, args ...interface{}) {
	nla.logger.Info(fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) Infoln(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Warn(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Warnf(format string, args ...interface{}) {
	nla.logger.Info(fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) Warnln(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Error(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Errorf(format string, args ...interface{}) {
	nla.logger.Info(fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) Errorln(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) info(args ...interface{}) {
	if len(args) == 0 {
		return
	}

	msg, ok := args[0].(string)
	if !ok {
		return
	}

	if len(args) > 1 {
		nla.logger.Info(msg, args[1:]...)
	} else if len(args) == 1 {
		nla.logger.Info(msg)
	}
}
