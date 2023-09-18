package notary

import (
	"fmt"

	"github.com/go-logr/logr"
	notationlog "github.com/notaryproject/notation-go/log"
)

func NotaryLoggerAdapter(logger logr.Logger) notationlog.Logger {
	return &notaryLoggerAdapter{
		logger: logger.V(4),
	}
}

type notaryLoggerAdapter struct {
	logger logr.Logger
}

func (nla *notaryLoggerAdapter) Debug(args ...interface{}) {
	nla.sprint(args...)
}

func (nla *notaryLoggerAdapter) Debugf(format string, args ...interface{}) {
	nla.sprintf(format, args...)
}

func (nla *notaryLoggerAdapter) Debugln(args ...interface{}) {
	nla.sprintln(args...)
}

func (nla *notaryLoggerAdapter) Info(args ...interface{}) {
	nla.sprint(args...)
}

func (nla *notaryLoggerAdapter) Infof(format string, args ...interface{}) {
	nla.sprintf(format, args...)
}

func (nla *notaryLoggerAdapter) Infoln(args ...interface{}) {
	nla.sprintln(args...)
}

func (nla *notaryLoggerAdapter) Warn(args ...interface{}) {
	nla.sprint(args...)
}

func (nla *notaryLoggerAdapter) Warnf(format string, args ...interface{}) {
	nla.sprintf(format, args...)
}

func (nla *notaryLoggerAdapter) Warnln(args ...interface{}) {
	nla.sprintln(args...)
}

func (nla *notaryLoggerAdapter) Error(args ...interface{}) {
	nla.logger.Error(fmt.Errorf(fmt.Sprint(args...)), "")
}

func (nla *notaryLoggerAdapter) Errorf(format string, args ...interface{}) {
	nla.logger.Error(fmt.Errorf(format, args...), "")
}

func (nla *notaryLoggerAdapter) Errorln(args ...interface{}) {
	nla.logger.Error(fmt.Errorf(fmt.Sprintln(args...)), "")
}

func (nla *notaryLoggerAdapter) info(args ...interface{}) {
	nla.logger.Info(fmt.Sprint(args...))
}

func (nla *notaryLoggerAdapter) sprintf(format string, args ...interface{}) {
	nla.logger.Info(fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) infoln(args ...interface{}) {
	nla.logger.Info(fmt.Sprintln(args...))
}
