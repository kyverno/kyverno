package logging

import (
	"fmt"

	"github.com/go-logr/logr"
)

const k8sStructuredEventMsg = "Event occurred"

// k8sEventTypeNormal reports whether keysAndValues are from client-go structured
// event logging and the event type is Normal (high-volume success-style events).
func k8sEventTypeNormal(keysAndValues []any) bool {
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		k, ok := keysAndValues[i].(string)
		if !ok || k != "type" {
			continue
		}
		switch v := keysAndValues[i+1].(type) {
		case string:
			return v == "Normal"
		default:
			return fmt.Sprint(v) == "Normal"
		}
	}
	return false
}

// klogVerbosityFilterSink wraps a logr.LogSink used for k8s.io/klog so that
// verbose client-go "Event occurred" logs for Normal events are not written
// when the user sets -v to 0 or 1. They remain available at -v >= 2.
type klogVerbosityFilterSink struct {
	delegate logr.LogSink
	maxV     int
}

func (k *klogVerbosityFilterSink) Init(info logr.RuntimeInfo) {
	k.delegate.Init(info)
}

func (k *klogVerbosityFilterSink) Enabled(level int) bool {
	return k.delegate.Enabled(level)
}

func (k *klogVerbosityFilterSink) Info(level int, msg string, keysAndValues ...any) {
	if k.maxV <= 1 && msg == k8sStructuredEventMsg && k8sEventTypeNormal(keysAndValues) {
		return
	}
	k.delegate.Info(level, msg, keysAndValues...)
}

func (k *klogVerbosityFilterSink) Error(err error, msg string, keysAndValues ...any) {
	k.delegate.Error(err, msg, keysAndValues...)
}

func (k *klogVerbosityFilterSink) WithValues(keysAndValues ...any) logr.LogSink {
	return &klogVerbosityFilterSink{delegate: k.delegate.WithValues(keysAndValues...), maxV: k.maxV}
}

func (k *klogVerbosityFilterSink) WithName(name string) logr.LogSink {
	return &klogVerbosityFilterSink{delegate: k.delegate.WithName(name), maxV: k.maxV}
}

var _ logr.LogSink = &klogVerbosityFilterSink{}

func (k *klogVerbosityFilterSink) WithCallDepth(depth int) logr.LogSink {
	if d, ok := k.delegate.(logr.CallDepthLogSink); ok {
		return &klogVerbosityFilterSink{delegate: d.WithCallDepth(depth), maxV: k.maxV}
	}
	return k
}

var _ logr.CallDepthLogSink = &klogVerbosityFilterSink{}

func wrapSinkForKlog(delegate logr.LogSink, maxV int) logr.LogSink {
	return &klogVerbosityFilterSink{delegate: delegate, maxV: maxV}
}
