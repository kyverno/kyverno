package log

import (
	"github.com/go-logr/logr"
)

// SetLogger sets a concrete logging implementation for all deferred Loggers.
func SetLogger(l logr.Logger) {
	Log.Fulfill(l)
}

// Log is the base logger used by kubebuilder.  It delegates
// to another logr.Logger.  You *must* call SetLogger to
// get any actual logging.
var Log = NewDelegatingLogger(NullLogger{})
