package utils

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/cel/trace"
)

// decisionTraceLogger is the name the decision traces are logged under, so they can be told
// apart from the rest of the admission controller's log.
const decisionTraceLogger = "decision-trace"

// LogDecisionTraces writes one log line per decision trace, under the decision-trace logger. A
// decision that did not pass (FAIL, ERROR) is logged at level 2, the default verbosity; PASS and
// SKIP are logged at level 3, since on a busy cluster nearly every request passes. A policy
// whose matchConstraints do not cover the resource never applied to it, and is logged at level
// 4: every request has some, since a policy on pods is also compiled for each pod controller.
//
// Policies only carry a trace when the admission controller compiles them with tracing on, so
// nil traces are skipped. It is meant to be run in its own goroutine, after the engine has
// answered, so that writing the log never holds up the admission response; for that reason it
// recovers from a panic rather than taking the admission controller down with it.
func LogDecisionTraces(logger logr.Logger, traces ...*trace.Decision) {
	logger = logger.WithName(decisionTraceLogger)
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Errorf("%v", r), "failed to log decision traces")
		}
	}()
	for _, d := range traces {
		if d == nil {
			continue
		}
		level := 3
		switch {
		case d.Verdict.Status == trace.VerdictFail || d.Verdict.Status == trace.VerdictError:
			level = 2
		case d.Verdict.Status == trace.VerdictSkip && !d.Scope.Applied && d.Scope.Reason != "":
			level = 4
		}
		logger.V(level).Info("policy decision", trace.LogValues(d)...)
	}
}
