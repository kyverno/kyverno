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
	nla.logger.V(4).Info(fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) Debugln(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Info(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Infof(format string, args ...interface{}) {
	nla.logger.V(4).Info(fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) Infoln(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Warn(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Warnf(format string, args ...interface{}) {
	nla.logger.V(4).Info(fmt.Sprintf(format, args...))
}

func (nla *notaryLoggerAdapter) Warnln(args ...interface{}) {
	nla.info(args...)
}

func (nla *notaryLoggerAdapter) Error(args ...interface{}) {
	nla.error(args...)
}

func (nla *notaryLoggerAdapter) Errorf(format string, args ...interface{}) {
	nla.logger.Error(fmt.Errorf(format, args...), "")
}

func (nla *notaryLoggerAdapter) Errorln(args ...interface{}) {
	nla.error(args...)
}

func (nla *notaryLoggerAdapter) info(args ...interface{}) {
	if len(args) == 0 {
		return
	}

	if len(args) == 1 {
		if str, ok := args[0].(string); ok {
			nla.logger.Info(str)
			return
		}
	}

	nla.logger.V(4).Info(fmt.Sprint(args...))
}

func (nla *notaryLoggerAdapter) error(args ...interface{}) {
	if len(args) == 0 {
		return
	}

	if len(args) == 1 {
		if str, ok := args[0].(string); ok {
			nla.logger.Error(fmt.Errorf(str), "")
			return
		}
	}

	nla.logger.Error(fmt.Errorf(fmt.Sprint(args...)), "")
}
