package trace

import "strings"

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
// A trace holds values read from resources, and a log is read by more people than the resources
// are. So every value is cut to maxValueLen. For a Secret decision, and for any values or errors
// that read from Secrets (e.g. via resource.Get or variables), no values or errors are logged
// except true and false: the expressions and which of them did not hold are still there, the data is not.
func LogValues(d *Decision) []any {
	if d == nil {
		return nil
	}
	l := newLogFormat(d)
	verdictResult := d.Verdict.Result
	verdictMsg := d.Verdict.Message
	if d.Verdict.Status == VerdictError {
		verdictResult = l.error(verdictResult, d.Verdict.Source)
		verdictMsg = l.error(verdictMsg, d.Verdict.Source)
	} else {
		verdictResult = l.value(verdictResult, d.Verdict.Source)
	}
	entry := LogEntry{
		Scope: d.Scope.Reason,
		Verdict: LogExpression{
			Status:     d.Verdict.Status,
			Expression: d.Verdict.Source,
			Result:     verdictResult,
			Message:    verdictMsg,
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
			valResult := v.Result
			if v.Status == VerdictError {
				valResult = l.error(valResult, v.Source)
			} else {
				valResult = l.value(valResult, v.Source)
			}
			entry.Validations = append(entry.Validations, LogExpression{
				Status:     v.Status,
				Expression: v.Source,
				Result:     valResult,
			})
		}
	}
	for _, m := range d.Mutations {
		mutation := LogExpression{
			Name:       m.Name,
			Expression: m.Source,
			Result:     l.value(m.Result, m.Source),
			Error:      l.error(m.Error, m.Source),
			NotApplied: m.NotApplied,
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
	secret     bool
	secretVars map[string]bool
}

func newLogFormat(d *Decision) logFormat {
	l := logFormat{
		secret:     strings.EqualFold(d.ResourceKind, "Secret"),
		secretVars: make(map[string]bool),
	}
	changed := true
	for changed {
		changed = false
		for _, v := range d.Variables {
			if !l.secretVars[v.Name] && (isSecretValue(v.Result) || l.isSecretExpr(v.Source)) {
				l.secretVars[v.Name] = true
				changed = true
			}
		}
	}
	return l
}

func isSecretValue(v string) bool {
	lower := strings.ToLower(v)
	return strings.Contains(lower, "kind:secret") || strings.Contains(lower, "kind: secret") ||
		strings.Contains(lower, `"kind":"secret"`) || strings.Contains(lower, `"kind": "secret"`)
}

func readsSecret(expr string) bool {
	lower := strings.ToLower(expr)
	return (strings.Contains(lower, "resource.get") || strings.Contains(lower, "resource.list")) &&
		(strings.Contains(lower, "'secret") || strings.Contains(lower, "\"secret") || strings.Contains(lower, "/secret"))
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func containsVar(expr, varName string) bool {
	prefixes := []string{"variables.", "variables.?"}
	for _, prefix := range prefixes {
		target := prefix + varName
		pos := 0
		for {
			idx := strings.Index(expr[pos:], target)
			if idx == -1 {
				break
			}
			start := pos + idx
			end := start + len(target)
			pos = start + 1

			if start > 0 && isIdentChar(expr[start-1]) {
				continue
			}
			if end < len(expr) && isIdentChar(expr[end]) {
				continue
			}
			return true
		}
	}
	if strings.Contains(expr, "variables['"+varName+"']") || strings.Contains(expr, `variables["`+varName+`"]`) {
		return true
	}
	return false
}

func (l logFormat) isSecretExpr(expr string) bool {
	if l.secret {
		return true
	}
	if readsSecret(expr) {
		return true
	}
	for name := range l.secretVars {
		if containsVar(expr, name) {
			return true
		}
	}
	return false
}

func (l logFormat) value(v string, expr string) string {
	if (l.secret || l.isSecretExpr(expr) || isSecretValue(v)) && v != "" && v != "true" && v != "false" {
		return redacted
	}
	return clip(v)
}

func (l logFormat) error(err string, expr string) string {
	if (l.secret || l.isSecretExpr(expr) || isSecretValue(err)) && err != "" {
		return redacted
	}
	return err
}

func (l logFormat) named(n NamedExpressionTrace) LogExpression {
	isSec := l.secret || l.secretVars[n.Name] || l.isSecretExpr(n.Source) || isSecretValue(n.Result)
	val := n.Result
	if isSec && val != "" && val != "true" && val != "false" {
		val = redacted
	} else {
		val = clip(val)
	}
	return LogExpression{Name: n.Name, Expression: n.Source, Result: val}
}

func (l logFormat) breakdown(e *LogExpression, et ExpressionTrace) {
	e.LoopValuesOmitted = et.LoopValuesOmitted
	isSecExpr := l.secret || l.isSecretExpr(et.Source)
	for _, n := range et.Nodes {
		errExpr := n.Expression
		if isSecExpr {
			errExpr = et.Source
		}
		e.Evaluated = append(e.Evaluated, LogNode{
			Expression: n.Expression,
			Value:      l.value(n.Value, n.Expression),
			Error:      l.error(n.Error, errExpr),
		})
	}
}
