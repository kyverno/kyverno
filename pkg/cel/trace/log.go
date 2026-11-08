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
		} else {
			verdictResult = clip(verdictResult)
		}
		if (verdictIsSec || isSecretValue(verdictMsg)) && verdictMsg != "" {
			verdictMsg = redacted
		} else {
			verdictMsg = clip(verdictMsg)
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
	mutationIsSecret := l.secret
	for _, m := range d.Mutations {
		if l.isSecretExpr(m.Source) {
			mutationIsSecret = true
		}
		result := m.Result
		if (mutationIsSecret || l.isSecretExpr(m.Source) || isSecretValue(result)) && result != "" && result != "true" && result != "false" {
			result = redacted
		} else {
			result = clip(result)
		}
		err := m.Error
		if (mutationIsSecret || l.isSecretExpr(m.Source) || isSecretValue(err)) && err != "" {
			err = redacted
		} else {
			err = clip(err)
		}
		mutation := LogExpression{
			Name:       m.Name,
			Expression: m.Source,
			Result:     result,
			Error:      err,
			NotApplied: m.NotApplied,
		}
		if m.Error != "" {
			l.breakdown(&mutation, m.ExpressionTrace)
			if mutationIsSecret {
				for i := range mutation.Evaluated {
					if mutation.Evaluated[i].Value != "" && mutation.Evaluated[i].Value != "true" && mutation.Evaluated[i].Value != "false" {
						mutation.Evaluated[i].Value = redacted
					}
					if mutation.Evaluated[i].Error != "" {
						mutation.Evaluated[i].Error = redacted
					}
				}
			}
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
	varAliases    map[string]bool
}

// newLogFormat initializes logFormat by discovering Secret GVR variables, Secret-bearing variables,
// variables aliases, and whether the admitted resource itself is a Secret.
func newLogFormat(d *Decision) logFormat {
	l := logFormat{
		secret:        strings.EqualFold(d.ResourceKind, "Secret"),
		secretVars:    make(map[string]bool),
		secretGVRVars: make(map[string]bool),
		varAliases:    map[string]bool{"variables": true},
	}
	changed := true
	for changed {
		changed = false
		for _, v := range d.Variables {
			if !l.varAliases[v.Name] && isVariablesAlias(v.Source, l.varAliases) {
				l.varAliases[v.Name] = true
				changed = true
			}
			if !l.secretGVRVars[v.Name] {
				if createsSecretGVR(v.Source, l.secretGVRVars, l.varAliases) ||
					isSecretResourceName(v.Result) ||
					isSecretResourceName(v.Source) ||
					isSecretGVRValue(v.Result) {
					l.secretGVRVars[v.Name] = true
					changed = true
				} else {
					for gvrVar := range l.secretGVRVars {
						if containsVar(v.Source, gvrVar, l.varAliases) {
							l.secretGVRVars[v.Name] = true
							changed = true
							break
						}
					}
				}
			}
			if !l.secretVars[v.Name] && (isSecretValue(v.Result) || l.isSecretExpr(v.Source)) {
				l.secretVars[v.Name] = true
				changed = true
			}
		}
	}
	return l
}

// isVerdictSecret reports whether a decision's verdict is associated with Secret data,
// either via admitted kind, verdict source expression, or associated match, mutation,
// validation, generation, or variable traces (especially for source-less verdicts).
func (l logFormat) isVerdictSecret(d *Decision) bool {
	if l.secret {
		return true
	}
	if d.Verdict.Source != "" {
		return l.isSecretExpr(d.Verdict.Source)
	}
	// For source-less verdicts (e.g. in MutatingPolicy where verdict has no source expression,
	// or when evaluation errors before/during match, variables, mutations, or generations):
	for _, m := range d.Match {
		if l.isSecretExpr(m.Source) {
			return true
		}
	}
	for _, m := range d.Mutations {
		if l.isSecretExpr(m.Source) {
			return true
		}
	}
	for _, v := range d.Validations {
		if l.isSecretExpr(v.Source) {
			return true
		}
	}
	for _, g := range d.Generations {
		if l.isSecretExpr(g.Source) {
			return true
		}
	}
	for _, v := range d.Variables {
		if l.secretVars[v.Name] || l.isSecretExpr(v.Source) {
			if d.Verdict.Status == VerdictError || isSecretValue(v.Result) {
				return true
			}
		}
	}
	return false
}

// isSecretValue reports whether v textually represents a Secret manifest or item.
func isSecretValue(v string) bool {
	lower := strings.ToLower(v)
	return strings.Contains(lower, "kind:secret") || strings.Contains(lower, "kind: secret") ||
		strings.Contains(lower, `"kind":"secret"`) || strings.Contains(lower, `"kind": "secret"`)
}

// isSecretResourceName reports whether name refers to the Secret kind or resource.
func isSecretResourceName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.Trim(lower, `"'`)
	return lower == "secret" || lower == "secrets" ||
		strings.HasPrefix(lower, "secrets/") || strings.HasPrefix(lower, "secret/") ||
		strings.HasSuffix(lower, "/secrets") || strings.HasSuffix(lower, "/secret")
}

// isSecretGVRValue reports whether val textually represents a Secret GVR string.
func isSecretGVRValue(val string) bool {
	lower := strings.ToLower(strings.TrimSpace(val))
	return strings.Contains(lower, "resource=secret") || strings.Contains(lower, "resource=secrets")
}

// literalString extracts the string value of a literal AST expression if present.
func literalString(e celast.Expr) (string, bool) {
	if e == nil || e.Kind() != celast.LiteralKind {
		return "", false
	}
	s, ok := e.AsLiteral().Value().(string)
	return s, ok
}

// astContainsVar reports whether AST expression e references varName under variables or its aliases.
func astContainsVar(e celast.Expr, sourceInfo *celast.SourceInfo, varName string, varAliases map[string]bool) bool {
	if e == nil {
		return false
	}
	if sourceInfo != nil {
		if s, err := cel.ExprToString(e, sourceInfo); err == nil {
			if containsVar(s, varName, varAliases) {
				return true
			}
		}
	}
	found := false
	celast.PreOrderVisit(e, celast.NewExprVisitor(func(sub celast.Expr) {
		if found {
			return
		}
		switch sub.Kind() {
		case celast.SelectKind:
			sel := sub.AsSelect()
			if sel.FieldName() == varName && sel.Operand().Kind() == celast.IdentKind {
				ident := strings.ToLower(sel.Operand().AsIdent())
				if ident == "variables" || (varAliases != nil && varAliases[ident]) {
					found = true
				}
			}
		case celast.CallKind:
			call := sub.AsCall()
			fn := call.FunctionName()
			if (fn == "_[_]" || fn == "_[?_]") && len(call.Args()) == 2 {
				firstArg := call.Args()[0]
				isVar := false
				if firstArg.Kind() == celast.IdentKind {
					ident := strings.ToLower(firstArg.AsIdent())
					if ident == "variables" || (varAliases != nil && varAliases[ident]) {
						isVar = true
					}
				}
				if isVar && call.Args()[1].Kind() == celast.LiteralKind {
					if str, ok := call.Args()[1].AsLiteral().Value().(string); ok && str == varName {
						found = true
					}
				}
			}
		}
	}))
	return found
}

// createsSecretGVRCall reports whether AST node e is a call to resource.ToGVR targeting Secrets.
func createsSecretGVRCall(e celast.Expr, secretGVRVars map[string]bool, varAliases map[string]bool) bool {
	if e == nil || e.Kind() != celast.CallKind {
		return false
	}
	call := e.AsCall()
	if !strings.EqualFold(call.FunctionName(), "ToGVR") {
		return false
	}
	if call.Target() == nil || call.Target().Kind() != celast.IdentKind || !strings.EqualFold(call.Target().AsIdent(), "resource") {
		return false
	}
	args := call.Args()
	if len(args) == 0 {
		return false
	}
	lastArg := args[len(args)-1]
	if str, ok := literalString(lastArg); ok && (isSecretResourceName(str) || isSecretGVRValue(str)) {
		return true
	}
	for gvrVar := range secretGVRVars {
		if astContainsVar(lastArg, nil, gvrVar, varAliases) {
			return true
		}
	}
	return false
}

// createsSecretGVR reports whether expr calls resource.ToGVR targeting Secrets.
func createsSecretGVR(expr string, secretGVRVars map[string]bool, varAliases map[string]bool) bool {
	ast, iss := celParserEnv.Parse(expr)
	if iss != nil && iss.Err() != nil {
		lower := strings.ToLower(expr)
		if !strings.Contains(lower, "togvr") {
			return false
		}
		if strings.Contains(lower, "'secret") || strings.Contains(lower, "\"secret") || strings.Contains(lower, "/secret") {
			return true
		}
		for gvrVar := range secretGVRVars {
			if containsVar(expr, gvrVar, varAliases) {
				return true
			}
		}
		return false
	}
	created := false
	sourceInfo := ast.NativeRep().SourceInfo()
	celast.PreOrderVisit(ast.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if created || e.Kind() != celast.CallKind {
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
		if str, ok := literalString(lastArg); ok && (isSecretResourceName(str) || isSecretGVRValue(str)) {
			created = true
			return
		}
		for gvrVar := range secretGVRVars {
			if astContainsVar(lastArg, sourceInfo, gvrVar, varAliases) {
				created = true
				return
			}
		}
	}))
	return created
}

// readsSecret reports whether expr calls resource.Get, resource.List, or globalContext.get for Secrets.
func (l logFormat) readsSecret(expr string) bool {
	ast, iss := celParserEnv.Parse(expr)
	if iss != nil && iss.Err() != nil {
		lower := strings.ToLower(expr)
		if strings.Contains(lower, "globalcontext.get") || strings.Contains(lower, "globalcontext.list") {
			return true
		}
		if !strings.Contains(lower, "resource.get") && !strings.Contains(lower, "resource.list") {
			return false
		}
		if strings.Contains(lower, "'secret") || strings.Contains(lower, "\"secret") || strings.Contains(lower, "/secret") {
			return true
		}
		for gvrVar := range l.secretGVRVars {
			if containsVar(expr, gvrVar, l.varAliases) {
				return true
			}
		}
		return false
	}
	reads := false
	sourceInfo := ast.NativeRep().SourceInfo()
	celast.PreOrderVisit(ast.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if reads || e.Kind() != celast.CallKind {
			return
		}
		call := e.AsCall()
		fn := strings.ToLower(call.FunctionName())
		if fn != "get" && fn != "list" {
			return
		}
		if call.Target() == nil || call.Target().Kind() != celast.IdentKind {
			return
		}
		targetIdent := call.Target().AsIdent()
		args := call.Args()
		if len(args) == 0 {
			return
		}

		if strings.EqualFold(targetIdent, "globalcontext") {
			reads = true
			return
		}

		if !strings.EqualFold(targetIdent, "resource") {
			return
		}

		// Check args[0]: can be GVR (e.g. resource.Get(gvr, ...), resource.Get('secrets', ...))
		if str, ok := literalString(args[0]); ok && (isSecretResourceName(str) || isSecretGVRValue(str)) {
			reads = true
			return
		}
		if createsSecretGVRCall(args[0], l.secretGVRVars, l.varAliases) {
			reads = true
			return
		}
		for gvrVar := range l.secretGVRVars {
			if astContainsVar(args[0], sourceInfo, gvrVar, l.varAliases) {
				reads = true
				return
			}
		}

		// Check args[1] if present: can be resource name in resource.Get(apiVersion, resource, ...)
		if len(args) >= 2 {
			if str, ok := literalString(args[1]); ok && (isSecretResourceName(str) || isSecretGVRValue(str)) {
				reads = true
				return
			}
			if createsSecretGVRCall(args[1], l.secretGVRVars, l.varAliases) {
				reads = true
				return
			}
			for gvrVar := range l.secretGVRVars {
				if astContainsVar(args[1], sourceInfo, gvrVar, l.varAliases) {
					reads = true
					return
				}
			}
		}
	}))
	return reads
}

// isIdentChar reports whether c is a valid CEL identifier character.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// isVariablesAlias reports whether expr is an alias reference to the variables object.
func isVariablesAlias(expr string, varAliases map[string]bool) bool {
	ast, iss := celParserEnv.Parse(expr)
	if iss == nil || iss.Err() == nil {
		e := ast.NativeRep().Expr()
		if e.Kind() == celast.IdentKind {
			ident := strings.ToLower(e.AsIdent())
			if ident == "variables" || (varAliases != nil && varAliases[ident]) {
				return true
			}
		} else if e.Kind() == celast.CallKind {
			call := e.AsCall()
			if strings.EqualFold(call.FunctionName(), "dyn") && len(call.Args()) == 1 {
				arg := call.Args()[0]
				if arg.Kind() == celast.IdentKind {
					ident := strings.ToLower(arg.AsIdent())
					if ident == "variables" || (varAliases != nil && varAliases[ident]) {
						return true
					}
				}
			}
		}
	}
	trimmed := strings.ToLower(strings.TrimSpace(expr))
	if trimmed == "variables" || (varAliases != nil && varAliases[trimmed]) {
		return true
	}
	if strings.HasPrefix(trimmed, "dyn(") && strings.HasSuffix(trimmed, ")") {
		inner := strings.TrimSpace(trimmed[4 : len(trimmed)-1])
		return inner == "variables" || (varAliases != nil && varAliases[inner])
	}
	return false
}

// containsVar reports whether expr references varName under the variables map or its aliases.
func containsVar(expr, varName string, varAliases map[string]bool) bool {
	ast, iss := celParserEnv.Parse(expr)
	if iss == nil || iss.Err() == nil {
		found := false
		celast.PreOrderVisit(ast.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
			if found {
				return
			}
			switch e.Kind() {
			case celast.SelectKind:
				sel := e.AsSelect()
				if sel.FieldName() == varName && sel.Operand().Kind() == celast.IdentKind {
					ident := strings.ToLower(sel.Operand().AsIdent())
					if ident == "variables" || (varAliases != nil && varAliases[ident]) {
						found = true
					}
				}
			case celast.CallKind:
				call := e.AsCall()
				fn := call.FunctionName()
				if (fn == "_[_]" || fn == "_[?_]") && len(call.Args()) == 2 {
					firstArg := call.Args()[0]
					isVar := false
					if firstArg.Kind() == celast.IdentKind {
						ident := strings.ToLower(firstArg.AsIdent())
						if ident == "variables" || (varAliases != nil && varAliases[ident]) {
							isVar = true
						}
					}
					if isVar && call.Args()[1].Kind() == celast.LiteralKind {
						if str, ok := call.Args()[1].AsLiteral().Value().(string); ok && str == varName {
							found = true
						}
					}
				}
			}
		}))
		if found {
			return true
		}
	}
	aliases := []string{"variables"}
	for a := range varAliases {
		if a != "variables" {
			aliases = append(aliases, a)
		}
	}
	for _, alias := range aliases {
		prefixes := []string{alias + ".", alias + ".?"}
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
		if strings.Contains(expr, alias+"['"+varName+"']") || strings.Contains(expr, alias+`["`+varName+`"]`) {
			return true
		}
	}
	return false
}

// hasDynamicVariableAccess reports whether expr dynamically indexes into variables or any of its aliases,
// e.g. variables[key], vars[key], or dyn(vars)[key] where key is not a literal string.
func hasDynamicVariableAccess(expr string, varAliases map[string]bool) bool {
	ast, iss := celParserEnv.Parse(expr)
	if iss == nil || iss.Err() == nil {
		hasDynamic := false
		celast.PreOrderVisit(ast.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
			if hasDynamic || e.Kind() != celast.CallKind {
				return
			}
			call := e.AsCall()
			fn := call.FunctionName()
			if (fn == "_[_]" || fn == "_[?_]") && len(call.Args()) == 2 {
				firstArg := call.Args()[0]
				isVar := false
				if firstArg.Kind() == celast.IdentKind {
					ident := strings.ToLower(firstArg.AsIdent())
					if ident == "variables" || (varAliases != nil && varAliases[ident]) {
						isVar = true
					}
				} else if firstArg.Kind() == celast.CallKind {
					inner := firstArg.AsCall()
					if strings.EqualFold(inner.FunctionName(), "dyn") && len(inner.Args()) == 1 {
						if inner.Args()[0].Kind() == celast.IdentKind {
							ident := strings.ToLower(inner.Args()[0].AsIdent())
							if ident == "variables" || (varAliases != nil && varAliases[ident]) {
								isVar = true
							}
						}
					}
				}
				if isVar && call.Args()[1].Kind() != celast.LiteralKind {
					hasDynamic = true
				}
			}
		}))
		if hasDynamic {
			return true
		}
	}
	lower := strings.ToLower(expr)
	aliases := []string{"variables"}
	for a := range varAliases {
		if a != "variables" {
			aliases = append(aliases, a)
		}
	}
	for _, alias := range aliases {
		if strings.Contains(lower, "dyn("+alias+")[") ||
			(strings.Contains(lower, alias+"[") && !strings.Contains(lower, alias+"['") && !strings.Contains(lower, alias+`["`)) {
			return true
		}
	}
	return false
}

// isSecretExpr reports whether expr reads a Secret or references a Secret-bearing variable.
func (l logFormat) isSecretExpr(expr string) bool {
	if l.secret {
		return true
	}
	if l.readsSecret(expr) {
		return true
	}
	if len(l.secretVars) > 0 && hasDynamicVariableAccess(expr, l.varAliases) {
		return true
	}
	for name := range l.secretVars {
		if containsVar(expr, name, l.varAliases) {
			return true
		}
	}
	return false
}

// value formats a value for logging, redacting it if it originates from a Secret.
func (l logFormat) value(v string, expr string) string {
	if (l.secret || l.isSecretExpr(expr) || isSecretValue(v)) && v != "" && v != "true" && v != "false" {
		return redacted
	}
	return clip(v)
}

// error formats an error for logging, redacting it if it originates from a Secret.
func (l logFormat) error(err string, expr string) string {
	if (l.secret || l.isSecretExpr(expr) || isSecretValue(err)) && err != "" {
		return redacted
	}
	return clip(err)
}

// named formats a NamedExpressionTrace, redacting its result if sensitive.
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

// breakdown populates the evaluated sub-expressions, redacting sensitive nodes.
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
