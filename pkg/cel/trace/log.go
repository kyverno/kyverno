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
// A trace holds values read from the resource, and a log is read by more people than the resource
// is. So every value is cut to maxValueLen, and for a Secret no value is logged at all except
// true and false: the expressions and which of them did not hold are still there, the data is not.
func LogValues(d *Decision) []any {
	if d == nil {
		return nil
	}
	l := logFormat{secret: d.ResourceKind == "Secret"}
	entry := LogEntry{
		Scope: d.Scope.Reason,
		Verdict: LogExpression{
			Status:     d.Verdict.Status,
			Expression: d.Verdict.Source,
			Result:     l.value(d.Verdict.Result, d.Verdict.ResultIsBoolean),
			Message:    l.value(d.Verdict.Message, false),
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
			Error:      l.value(m.Error, false),
			NotApplied: l.value(m.NotApplied, false),
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

// logFormat holds what decides how a value is logged.
type logFormat struct {
	secret bool
}

// value formats a CEL evaluation result for logging. In a secret trace, only genuine CEL boolean
// results (flagged by isBoolean) are kept; all other non-empty values are replaced with
// "(redacted)". This prevents a Secret field whose decoded value is literally "true" or "false"
// from bypassing redaction.
func (l logFormat) value(v string, isBoolean bool) string {
	if l.secret && v != "" && !isBoolean {
		return redacted
	}
	return clip(v)
}

func (l logFormat) named(n NamedExpressionTrace) LogExpression {
	return LogExpression{Name: n.Name, Expression: n.Source, Result: l.value(n.Result, n.ResultIsBoolean)}
}

func (l logFormat) breakdown(e *LogExpression, et ExpressionTrace) {
	e.LoopValuesOmitted = et.LoopValuesOmitted
	for _, n := range et.Nodes {
		e.Evaluated = append(e.Evaluated, LogNode{Expression: n.Expression, Value: l.value(n.Value, n.ValueIsBoolean), Error: l.value(n.Error, false)})
	}
}
