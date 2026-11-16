package utils

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLog returns a logger that keeps messages up to the given verbosity, and the lines it
// has written so far, decoded.
func captureLog(t *testing.T, verbosity int) (logr.Logger, func() []map[string]any) {
	t.Helper()
	var lines []map[string]any
	logger := funcr.NewJSON(func(obj string) {
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(obj), &line))
		lines = append(lines, line)
	}, funcr.Options{Verbosity: verbosity})
	return logger, func() []map[string]any { return lines }
}

func decision(policy, status string) *trace.Decision {
	return &trace.Decision{
		PolicyName: policy,
		PolicyKind: "ValidatingPolicy",
		Verdict:    trace.VerdictTrace{Status: status, ExpressionTrace: trace.ExpressionTrace{Source: "object.spec.replicas > 1", Result: "false"}},
	}
}

func TestLogDecisionTraces(t *testing.T) {
	// a policy whose matchConstraints do not cover the resource, such as the copy of a pod
	// policy compiled for deployments when a pod is admitted
	notApplied := decision("does-not-apply", trace.VerdictSkip)
	notApplied.Scope = trace.ScopeTrace{Applied: false, Reason: "kind Pod is not covered by the policy's resourceRules"}
	all := []*trace.Decision{
		decision("passes", trace.VerdictPass),
		nil,
		decision("fails", trace.VerdictFail),
		decision("errors", trace.VerdictError),
		decision("skips", trace.VerdictSkip),
		notApplied,
	}
	tests := []struct {
		name      string
		verbosity int
		want      []string
	}{{
		name:      "the default verbosity logs the decisions that did not pass",
		verbosity: 2,
		want:      []string{"fails", "errors"},
	}, {
		name:      "verbosity 3 logs every decision of a policy that applied",
		verbosity: 3,
		want:      []string{"passes", "fails", "errors", "skips"},
	}, {
		name:      "verbosity 4 also logs the policies that did not apply",
		verbosity: 4,
		want:      []string{"passes", "fails", "errors", "skips", "does-not-apply"},
	}, {
		name:      "below the default verbosity nothing is logged",
		verbosity: 1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, lines := captureLog(t, tt.verbosity)
			LogDecisionTraces(logger, all...)
			var got []string
			for _, line := range lines() {
				assert.Equal(t, "decision-trace", line["logger"])
				assert.Equal(t, "policy decision", line["msg"])
				assert.Equal(t, "ValidatingPolicy", line["policyKind"])
				logged, ok := line["trace"].(map[string]any)
				require.True(t, ok, "the trace is logged as a structured value")
				verdict, ok := logged["verdict"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, line["verdict"], verdict["status"])
				assert.Equal(t, "object.spec.replicas > 1", verdict["expression"])
				got = append(got, line["policy"].(string))
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// panickingSink panics on every Info and records every Error.
type panickingSink struct {
	errors *[]string
}

func (panickingSink) Init(logr.RuntimeInfo)                 {}
func (panickingSink) Enabled(int) bool                      { return true }
func (panickingSink) Info(int, string, ...any)              { panic("the log is broken") }
func (s panickingSink) Error(_ error, msg string, _ ...any) { *s.errors = append(*s.errors, msg) }
func (s panickingSink) WithValues(...any) logr.LogSink      { return s }
func (s panickingSink) WithName(string) logr.LogSink        { return s }

// TestLogDecisionTraces_RecoversFromAPanic: the traces are logged in a goroutine of the
// admission controller, where an unrecovered panic would end the process.
func TestLogDecisionTraces_RecoversFromAPanic(t *testing.T) {
	var errors []string
	assert.NotPanics(t, func() {
		LogDecisionTraces(logr.New(panickingSink{errors: &errors}), decision("fails", trace.VerdictFail))
	})
	assert.Equal(t, []string{"failed to log decision traces"}, errors)
}
