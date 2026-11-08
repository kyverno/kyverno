package trace

// redacted stands in for a value that is left out of a logged trace.
const redacted = "(redacted)"

// LogEntry is a Decision in the form it is written to a log: the same layers Render prints, as
// one structured value instead of lines of text. It carries less than the Decision does, the same
// way Render prints less: the per-node breakdown is kept only where something did not pass.
type LogEntry struct {
	Scope       string          `json:"scope,omitempty"`
	Match       []LogExpression `json:"match,omitempty"`
	Variables   []LogExpression `json:"variables,omitempty"`
	Validations []LogExpression `json:"validations,omitempty"`
	Mutations   []LogExpression `json:"mutations,omitempty"`
	Verdict     LogExpression   `json:"verdict"`
}

// LogExpression is one traced expression of a LogEntry.
type LogExpression struct {
	Name       string `json:"name,omitempty"`
	Status     string `json:"status,omitempty"`
	Expression string `json:"expression,omitempty"`
	Result     string `json:"result,omitempty"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
	NotApplied string `json:"notApplied,omitempty"`
	// Evaluated is the per-node breakdown, see LogEntry for when it is kept.
	Evaluated         []LogNode `json:"evaluated,omitempty"`
	LoopValuesOmitted bool      `json:"loopValuesOmitted,omitempty"`
}

// LogNode is one sub-expression of a breakdown and the value, or error, it resolved to.
type LogNode struct {
	Expression string `json:"expression"`
	Value      string `json:"value,omitempty"`
	Error      string `json:"error,omitempty"`
}

// LogValues returns d as the key/value pairs of one structured log line: the policy, the verdict
// and the trace itself as a LogEntry. The resource is left to the caller's logger, which in an
// admission handler already carries the request. A nil decision returns nil.
//
// A trace holds values read from resources, and a log is read by more people than the resource
// is. So every value is cut to maxValueLen, and for a Secret or a trace that reads one no value is
// logged at all except true and false: the expressions and which of them did not hold are still
// there, the data is not.
func LogValues(d *Decision) []any {
	if d == nil {
		return nil
	}
	l := logFormat{sensitive: d.IsSensitive()}
	entry := LogEntry{
		Scope: d.Scope.Reason,
		Verdict: LogExpression{
			Status:     d.Verdict.Status,
			Expression: d.Verdict.Source,
			Result:     l.value(d.Verdict.Result, d.Verdict.ResultIsBoolean),
			Message:    l.text(d.Verdict.Message),
		},
	}
	if d.Verdict.Status != VerdictPass {
		l.breakdown(&entry.Verdict, d.Verdict.ExpressionTrace)
	}
	for _, m := range d.Match {
		entry.Match = append(entry.Match, l.named(m))
	}
	for _, v := range d.Variables {
		entry.Variables = append(entry.Variables, l.named(v))
	}
	// with a single validation the list would only repeat the verdict
	if len(d.Validations) > 1 {
		for _, v := range d.Validations {
			entry.Validations = append(entry.Validations, LogExpression{
				Status:     v.Status,
				Expression: v.Source,
				Result:     l.value(v.Result, v.ResultIsBoolean),
			})
		}
	}
	for _, m := range d.Mutations {
		mutation := LogExpression{
			Name:       m.Name,
			Expression: m.Source,
			Result:     l.value(m.Result, m.ResultIsBoolean),
			Error:      l.text(m.Error),
			NotApplied: l.text(m.NotApplied),
		}
		if m.Error != "" {
			l.breakdown(&mutation, m.ExpressionTrace)
		}
		entry.Mutations = append(entry.Mutations, mutation)
	}
	return []any{
		"policy", d.PolicyName,
		"policyKind", d.PolicyKind,
		"verdict", d.Verdict.Status,
		"trace", entry,
	}
}

// IsSensitive reports whether any part of the decision trace may contain Secret data.
func (d *Decision) IsSensitive() bool {
	return d != nil && (d.ResourceKind == "Secret" || decisionHasSecretRead(d))
}

// logFormat holds what decides how a value is logged.
type logFormat struct {
	sensitive bool
}

// decisionHasSecretRead reports whether a trace section was compiled as Secret-reading.
func decisionHasSecretRead(d *Decision) bool {
	if d.Sensitive {
		return true
	}
	if d.Verdict.HasSecretRead {
		return true
	}
	for _, expression := range d.Match {
		if expression.HasSecretRead {
			return true
		}
	}
	for _, expression := range d.Variables {
		if expression.HasSecretRead {
			return true
		}
	}
	for _, expression := range d.Validations {
		if expression.HasSecretRead {
			return true
		}
	}
	for _, expression := range d.Mutations {
		if expression.HasSecretRead {
			return true
		}
	}
	for _, expression := range d.Generations {
		if expression.HasSecretRead {
			return true
		}
	}
	return false
}

// value formats an evaluated CEL value, preserving only boolean results in sensitive traces.
func (l logFormat) value(v string, isBoolean bool) string {
	if l.sensitive && v != "" && !(isBoolean && (v == "true" || v == "false")) {
		return redacted
	}
	return clip(v)
}

// text formats free-form trace text and errors, redacting them for sensitive traces.
func (l logFormat) text(v string) string {
	if l.sensitive && v != "" {
		return redacted
	}
	return clip(v)
}

// named converts an expression trace to its log representation.
func (l logFormat) named(n NamedExpressionTrace) LogExpression {
	return LogExpression{Name: n.Name, Expression: n.Source, Result: l.value(n.Result, n.ResultIsBoolean)}
}

// breakdown appends the evaluated nodes to an expression's log representation.
func (l logFormat) breakdown(e *LogExpression, et ExpressionTrace) {
	e.LoopValuesOmitted = et.LoopValuesOmitted
	for _, n := range et.Nodes {
		e.Evaluated = append(e.Evaluated, LogNode{
			Expression: n.Expression,
			Value:      l.value(n.Value, n.ValueIsBoolean),
			Error:      l.text(n.Error),
		})
	}
}
