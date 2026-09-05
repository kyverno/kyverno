package logging

import (
	"testing"

	"github.com/go-logr/logr"
)

func TestK8sEventTypeNormal(t *testing.T) {
	t.Parallel()
	if !k8sEventTypeNormal([]any{"type", "Normal", "reason", "PolicyApplied"}) {
		t.Fatal("expected Normal event")
	}
	if k8sEventTypeNormal([]any{"type", "Warning", "reason", "Failed"}) {
		t.Fatal("did not expect Warning as Normal")
	}
	if k8sEventTypeNormal([]any{"reason", "PolicyApplied"}) {
		t.Fatal("missing type must not be treated as Normal")
	}
}

type stubLogSink struct {
	infoCalls int
	lastMsg   string
}

func (s *stubLogSink) Init(logr.RuntimeInfo) {}

func (s *stubLogSink) Enabled(int) bool { return true }

func (s *stubLogSink) Info(_ int, msg string, _ ...any) {
	s.infoCalls++
	s.lastMsg = msg
}

func (s *stubLogSink) Error(_ error, _ string, _ ...any) {}

func (s *stubLogSink) WithValues(...any) logr.LogSink { return s }

func (s *stubLogSink) WithName(string) logr.LogSink { return s }

func (s *stubLogSink) WithCallDepth(int) logr.LogSink { return s }

var (
	_ logr.LogSink          = &stubLogSink{}
	_ logr.CallDepthLogSink = &stubLogSink{}
)

func TestKlogVerbosityFilterSinkSuppressesNormalEventsAtLowV(t *testing.T) {
	t.Parallel()
	stub := &stubLogSink{}
	wrapped := wrapSinkForKlog(stub, 1)
	wrapped.Info(0, k8sStructuredEventMsg, "type", "Normal", "reason", "PolicyApplied")
	if stub.infoCalls != 0 {
		t.Fatalf("expected filter to drop Normal event at maxV=1, got %d calls", stub.infoCalls)
	}
	wrapped.Info(0, k8sStructuredEventMsg, "type", "Warning", "reason", "Failed")
	if stub.infoCalls != 1 || stub.lastMsg != k8sStructuredEventMsg {
		t.Fatalf("expected Warning event through, calls=%d msg=%q", stub.infoCalls, stub.lastMsg)
	}
	wrapped.Info(0, "other message")
	if stub.infoCalls != 2 || stub.lastMsg != "other message" {
		t.Fatalf("expected other message through, calls=%d msg=%q", stub.infoCalls, stub.lastMsg)
	}
}

func TestKlogVerbosityFilterSinkAllowsNormalEventsAtHigherV(t *testing.T) {
	t.Parallel()
	stub := &stubLogSink{}
	wrapped := wrapSinkForKlog(stub, 2)
	wrapped.Info(0, k8sStructuredEventMsg, "type", "Normal", "reason", "PolicyApplied")
	if stub.infoCalls != 1 {
		t.Fatalf("expected Normal event at maxV=2, got %d calls", stub.infoCalls)
	}
}
