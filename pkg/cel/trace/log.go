package trace

import (
	"strings"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
)

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
	verdictIsSec := l.isVerdictSecret(d)
	verdictResult := d.Verdict.Result
	verdictMsg := d.Verdict.Message
	if d.Verdict.Status == VerdictError {
		if (verdictIsSec || isSecretValue(verdictResult)) && verdictResult != "" {
			verdictResult = redacted
		}
		if (verdictIsSec || isSecretValue(verdictMsg)) && verdictMsg != "" {
			verdictMsg = redacted
		}
	} else {
		if (verdictIsSec || isSecretValue(verdictResult)) && verdictResult != "" && verdictResult != "true" && verdictResult != "false" {
			verdictResult = redacted
		} else {
			verdictResult = clip(verdictResult)
		}
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

// celParserEnv is used for syntactic AST inspection of CEL expressions.
var celParserEnv, _ = cel.NewEnv(cel.OptionalTypes())

// logFormat holds what decides how a value is logged.
type logFormat struct {
	secret        bool
	secretVars    map[string]bool
	secretGVRVars map[string]bool
}

func newLogFormat(d *Decision) logFormat {
	l := logFormat{
		secret:        strings.EqualFold(d.ResourceKind, "Secret"),
		secretVars:    make(map[string]bool),
		secretGVRVars: make(map[string]bool),
	}
	for _, v := range d.Variables {
		if createsSecretGVR(v.Source) {
			l.secretGVRVars[v.Name] = true
		}
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

func (l logFormat) isVerdictSecret(d *Decision) bool {
	if l.secret {
		return true
	}
	if d.Verdict.Source != "" {
		return l.isSecretExpr(d.Verdict.Source)
	}
	// For source-less verdicts (e.g. in MutatingPolicy where verdict has no source expression),
	// check if any mutation that ran or errored is a secret expression.
	for _, m := range d.Mutations {
		if l.isSecretExpr(m.Source) {
			return true
		}
	}
	return false
}

func isSecretValue(v string) bool {
	lower := strings.ToLower(v)
	return strings.Contains(lower, "kind:secret") || strings.Contains(lower, "kind: secret") ||
		strings.Contains(lower, `"kind":"secret"`) || strings.Contains(lower, `"kind": "secret"`)
}

func isSecretResourceName(name string) bool {
	lower := strings.ToLower(name)
	return lower == "secret" || lower == "secrets" || strings.HasPrefix(lower, "secrets/") || strings.HasPrefix(lower, "secret/")
}

// createsSecretGVR reports whether expr calls resource.ToGVR targeting Secrets.
func createsSecretGVR(expr string) bool {
	ast, iss := celParserEnv.Parse(expr)
	if iss != nil && iss.Err() != nil {
		lower := strings.ToLower(expr)
		return strings.Contains(lower, "togvr") && (strings.Contains(lower, "'secret") || strings.Contains(lower, "\"secret"))
	}
	created := false
	celast.PreOrderVisit(ast.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if e.Kind() != celast.CallKind {
			return
		}
		call := e.AsCall()
		if !strings.EqualFold(call.FunctionName(), "ToGVR") {
			return
		}
		if call.Target() == nil || call.Target().Kind() != celast.IdentKind || !strings.EqualFold(call.Target().AsIdent(), "resource") {
			return
		}
		args := call.Args()
		if len(args) == 0 {
			return
		}
		lastArg := args[len(args)-1]
		if lastArg.Kind() == celast.LiteralKind {
			if str, ok := lastArg.AsLiteral().Value().(string); ok && isSecretResourceName(str) {
				created = true
			}
		}
	}))
	return created
}

// readsSecret reports whether expr calls resource.Get or resource.List for Secrets.
func (l logFormat) readsSecret(expr string) bool {
	ast, iss := celParserEnv.Parse(expr)
	if iss != nil && iss.Err() != nil {
		lower := strings.ToLower(expr)
		return (strings.Contains(lower, "resource.get") || strings.Contains(lower, "resource.list")) &&
			(strings.Contains(lower, "'secret") || strings.Contains(lower, "\"secret") || strings.Contains(lower, "/secret"))
	}
	reads := false
	sourceInfo := ast.NativeRep().SourceInfo()
	celast.PreOrderVisit(ast.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if e.Kind() != celast.CallKind {
			return
		}
		call := e.AsCall()
		fn := strings.ToLower(call.FunctionName())
		if fn != "get" && fn != "list" {
			return
		}
		if call.Target() == nil || call.Target().Kind() != celast.IdentKind || !strings.EqualFold(call.Target().AsIdent(), "resource") {
			return
		}
		args := call.Args()
		if len(args) == 0 {
			return
		}

		// Pattern 1: resource.Get('v1', 'secrets', ...) or resource.List('v1', 'secrets', ...)
		if len(args) >= 2 && args[0].Kind() == celast.LiteralKind {
			if _, ok := args[0].AsLiteral().Value().(string); ok {
				if args[1].Kind() == celast.LiteralKind {
					if resName, ok := args[1].AsLiteral().Value().(string); ok && isSecretResourceName(resName) {
						reads = true
						return
					}
				}
			}
		}

		// Pattern 2: resource.Get(gvr, ...) or resource.List(gvr, ...)
		gvrArg := args[0]
		if gvrArg.Kind() == celast.CallKind {
			innerCall := gvrArg.AsCall()
			if strings.EqualFold(innerCall.FunctionName(), "ToGVR") {
				innerArgs := innerCall.Args()
				if len(innerArgs) > 0 {
					lastArg := innerArgs[len(innerArgs)-1]
					if lastArg.Kind() == celast.LiteralKind {
						if kindName, ok := lastArg.AsLiteral().Value().(string); ok && isSecretResourceName(kindName) {
							reads = true
							return
						}
					}
				}
			}
		}

		for gvrVar := range l.secretGVRVars {
			gvrExprStr, err := cel.ExprToString(gvrArg, sourceInfo)
			if err == nil && containsVar(gvrExprStr, gvrVar) {
				reads = true
				return
			}
		}
	}))
	return reads
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
	if l.readsSecret(expr) {
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
