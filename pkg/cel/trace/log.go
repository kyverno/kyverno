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
	} else {
		if (verdictIsSec || isSecretValue(verdictResult)) && verdictResult != "" && verdictResult != "true" && verdictResult != "false" {
			verdictResult = redacted
		} else {
			verdictResult = clip(verdictResult)
		}
	}
	if (verdictIsSec || isSecretValue(verdictMsg)) && verdictMsg != "" {
		verdictMsg = redacted
	} else {
		verdictMsg = clip(verdictMsg)
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
		l.breakdown(&entry.Verdict, d.Verdict.ExpressionTrace, verdictIsSec)
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
			l.breakdown(&mutation, m.ExpressionTrace, mutationIsSecret)
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
	safeGVRVars   map[string]bool
	varAliases    map[string]bool
}

// newLogFormat initializes logFormat by discovering Secret GVR variables, Secret-bearing variables,
// safe GVR variables, variables aliases, and whether the admitted resource itself is a Secret.
func newLogFormat(d *Decision) logFormat {
	l := logFormat{
		secret:        strings.EqualFold(d.ResourceKind, "Secret"),
		secretVars:    make(map[string]bool),
		secretGVRVars: make(map[string]bool),
		safeGVRVars:   make(map[string]bool),
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
			if !l.safeGVRVars[v.Name] && isStaticNonSecretGVR(v.Source) {
				l.safeGVRVars[v.Name] = true
				changed = true
			}
			if len(l.secretVars) > 0 && l.varAliases[v.Name] && !l.secretVars[v.Name] {
				l.secretVars[v.Name] = true
				changed = true
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
	if d.Verdict.Source != "" && l.isSecretExpr(d.Verdict.Source) {
		return true
	}
	if l.isPlaceholderErrorSecret(d.Verdict.Message) {
		return true
	}
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
		if l.isSecretExpr(g.Source) || l.isPlaceholderErrorSecret(g.Error) {
			return true
		}
		for _, gen := range g.Generated {
			if strings.HasPrefix(gen, "Secret ") || gen == "Secret" {
				return true
			}
		}
	}
	for _, v := range d.Variables {
		if l.secretVars[v.Name] || l.isSecretExpr(v.Source) {
			return true
		}
	}
	return false
}

// isPlaceholderErrorSecret reports whether errStr arises from a template placeholder
// expression that accesses Secret data or Secret variables.
func (l logFormat) isPlaceholderErrorSecret(errStr string) bool {
	if errStr == "" {
		return false
	}
	if isSecretValue(errStr) {
		return true
	}
	for name := range l.secretVars {
		if strings.Contains(errStr, name) {
			return true
		}
	}
	for _, quote := range []string{`placeholder "`, "placeholder `", "placeholder '"} {
		remaining := errStr
		for {
			idx := strings.Index(remaining, quote)
			if idx == -1 {
				break
			}
			start := idx + len(quote)
			rest := remaining[start:]
			qChar := quote[len(quote)-1]
			end := -1
			for i := 0; i < len(rest); i++ {
				if rest[i] == '\\' {
					i++
					continue
				}
				if rest[i] == qChar {
					end = i
					break
				}
			}
			if end == -1 {
				break
			}
			expr := rest[:end]
			if qChar == '"' {
				expr = strings.ReplaceAll(expr, `\"`, `"`)
			}
			if l.isSecretExpr(expr) {
				return true
			}
			remaining = rest[end+1:]
		}
	}
	return false
}

// isSecretValue reports whether v textually represents a Secret manifest or item,
// matching the kind field as an exact "Secret" token rather than prefixing CRDs like SecretStore.
func isSecretValue(v string) bool {
	lower := strings.ToLower(v)
	pos := 0
	for {
		idx := strings.Index(lower[pos:], "kind")
		if idx == -1 {
			break
		}
		start := pos + idx + 4 // after "kind"
		pos = start
		if idx > 0 {
			prev := lower[pos-5]
			if isIdentChar(prev) && prev != '"' && prev != '\'' {
				continue
			}
		}
		rem := lower[start:]
		i := 0
		if i < len(rem) && (rem[i] == '"' || rem[i] == '\'') {
			i++
		}
		for i < len(rem) && (rem[i] == ' ' || rem[i] == '\t') {
			i++
		}
		if i >= len(rem) || rem[i] != ':' {
			continue
		}
		i++ // skip ':'
		for i < len(rem) && (rem[i] == ' ' || rem[i] == '\t') {
			i++
		}
		hasQuote := false
		var quoteChar byte
		if i < len(rem) && (rem[i] == '"' || rem[i] == '\'') {
			hasQuote = true
			quoteChar = rem[i]
			i++
		}
		if strings.HasPrefix(rem[i:], "secret") {
			after := i + 6
			if hasQuote {
				if after < len(rem) && rem[after] == quoteChar {
					after++
				} else {
					continue
				}
			}
			if after < len(rem) {
				next := rem[after]
				if isIdentChar(next) || next == '-' {
					continue
				}
			}
			return true
		}
	}
	return false
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
	for _, pattern := range []string{"resource=secrets", "resource=secret"} {
		pos := 0
		for {
			idx := strings.Index(lower[pos:], pattern)
			if idx == -1 {
				break
			}
			end := pos + idx + len(pattern)
			pos = end
			if end < len(lower) {
				next := lower[end]
				if isIdentChar(next) || next == '-' {
					continue
				}
			}
			return true
		}
	}
	return false
}

// literalString extracts the string value of a literal AST expression if present.
func literalString(e celast.Expr) (string, bool) {
	if e == nil || e.Kind() != celast.LiteralKind {
		return "", false
	}
	s, ok := e.AsLiteral().Value().(string)
	return s, ok
}

// isVariablesExpr reports whether AST node e evaluates to the variables root or any of its aliases,
// including dyn-wrapped forms like dyn(variables) or dyn(alias).
func isVariablesExpr(e celast.Expr, varAliases map[string]bool) bool {
	if e == nil {
		return false
	}
	switch e.Kind() {
	case celast.IdentKind:
		ident := e.AsIdent()
		return ident == "variables" || (varAliases != nil && varAliases[ident])
	case celast.CallKind:
		call := e.AsCall()
		if strings.EqualFold(call.FunctionName(), "dyn") && len(call.Args()) == 1 {
			return isVariablesExpr(call.Args()[0], varAliases)
		}
	}
	return false
}

// referencesVariablesRoot reports whether AST node e references the variables root or any of its aliases.
func referencesVariablesRoot(e celast.Expr, varAliases map[string]bool) bool {
	if e == nil {
		return false
	}
	refs := false
	celast.PreOrderVisit(e, celast.NewExprVisitor(func(sub celast.Expr) {
		if refs {
			return
		}
		if sub.Kind() == celast.IdentKind {
			ident := sub.AsIdent()
			if ident == "variables" || (varAliases != nil && varAliases[ident]) {
				refs = true
			}
		}
	}))
	return refs
}

// isBareVariableRef reports whether AST expression e is a bare reference to varName
// under variables or its aliases (e.g. varName, variables.varName, dyn(variables)["varName"]),
// and NOT a composite expression such as a ternary or collection containing varName.
func isBareVariableRef(e celast.Expr, varName string, varAliases map[string]bool) bool {
	if e == nil {
		return false
	}
	switch e.Kind() {
	case celast.IdentKind:
		return e.AsIdent() == varName
	case celast.SelectKind:
		sel := e.AsSelect()
		return sel.FieldName() == varName && isVariablesExpr(sel.Operand(), varAliases)
	case celast.CallKind:
		call := e.AsCall()
		fn := call.FunctionName()
		if strings.EqualFold(fn, "dyn") && len(call.Args()) == 1 {
			return isBareVariableRef(call.Args()[0], varName, varAliases)
		}
		if (fn == "_[_]" || fn == "_[?_]") && len(call.Args()) == 2 {
			if isVariablesExpr(call.Args()[0], varAliases) && call.Args()[1].Kind() == celast.LiteralKind {
				if str, ok := call.Args()[1].AsLiteral().Value().(string); ok && str == varName {
					return true
				}
			}
		}
	}
	return false
}

// astContainsVar reports whether AST expression e references varName under variables or its aliases,
// including through collections, dyn wrapping, or property accesses.
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
			if sel.FieldName() == varName && (isVariablesExpr(sel.Operand(), varAliases) || referencesVariablesRoot(sel.Operand(), varAliases)) {
				found = true
			}
		case celast.CallKind:
			call := sub.AsCall()
			fn := call.FunctionName()
			if (fn == "_[_]" || fn == "_[?_]") && len(call.Args()) == 2 {
				if (isVariablesExpr(call.Args()[0], varAliases) || referencesVariablesRoot(call.Args()[0], varAliases)) && call.Args()[1].Kind() == celast.LiteralKind {
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
	if str, ok := literalString(lastArg); ok {
		return isSecretResourceName(str) || isSecretGVRValue(str)
	}
	for gvrVar := range secretGVRVars {
		if astContainsVar(lastArg, nil, gvrVar, varAliases) {
			return true
		}
	}
	// Dynamic selector in resource.ToGVR: conservative tainting
	return true
}

// isStaticNonSecretGVRCall reports whether AST node e is a call to resource.ToGVR with a statically known non-Secret target.
func isStaticNonSecretGVRCall(e celast.Expr) bool {
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
	if str, ok := literalString(lastArg); ok {
		return !isSecretResourceName(str) && !isSecretGVRValue(str)
	}
	return false
}

// isStaticNonSecretGVR reports whether expr statically creates a non-Secret GVR via resource.ToGVR.
func isStaticNonSecretGVR(expr string) bool {
	ast, iss := celParserEnv.Parse(expr)
	if iss != nil && iss.Err() != nil {
		return false
	}
	return isStaticNonSecretGVRCall(ast.NativeRep().Expr())
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
		if str, ok := literalString(lastArg); ok {
			if isSecretResourceName(str) || isSecretGVRValue(str) {
				created = true
			}
			return
		}
		for gvrVar := range secretGVRVars {
			if astContainsVar(lastArg, sourceInfo, gvrVar, varAliases) {
				created = true
				return
			}
		}
		// Dynamic selector in resource.ToGVR: conservative tainting
		created = true
	}))
	return created
}

// isAPIVersion reports whether s textually represents a Kubernetes API version (e.g. "v1", "apps/v1").
func isAPIVersion(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	parts := strings.Split(s, "/")
	last := parts[len(parts)-1]
	return len(last) >= 2 && last[0] == 'v' && last[1] >= '0' && last[1] <= '9'
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

		// Case 1: args[0] is a literal string
		if str, ok := literalString(args[0]); ok {
			if isSecretResourceName(str) || isSecretGVRValue(str) {
				reads = true
				return
			}
			if isAPIVersion(str) && len(args) >= 2 {
				// args[0] is apiVersion, so args[1] is the resource selector
				if str1, ok1 := literalString(args[1]); ok1 {
					if isSecretResourceName(str1) || isSecretGVRValue(str1) {
						reads = true
						return
					}
					// args[1] is a known literal non-secret resource (e.g. 'configmaps', 'pods')
					return
				}
				if isStaticNonSecretGVRCall(args[1]) {
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
				for safeVar := range l.safeGVRVars {
					if isBareVariableRef(args[1], safeVar, l.varAliases) {
						return
					}
				}
				// args[1] is a dynamic resource selector (e.g. cond ? 'secrets' : 'configmaps')
				// Unknown dynamic selectors need conservative tainting.
				reads = true
				return
			}
			// args[0] is a literal non-secret GVR (e.g. 'v1/configmaps')
			return
		}

		// Case 2: args[0] is not a literal string
		if isStaticNonSecretGVRCall(args[0]) {
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
		for safeVar := range l.safeGVRVars {
			if isBareVariableRef(args[0], safeVar, l.varAliases) {
				return
			}
		}
		// args[0] is an unknown dynamic selector (computed GVR or dynamic call): conservative tainting
		reads = true
		return
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
		if isVariablesExpr(ast.NativeRep().Expr(), varAliases) {
			return true
		}
	}
	trimmed := strings.TrimSpace(expr)
	if trimmed == "variables" || (varAliases != nil && varAliases[trimmed]) {
		return true
	}
	for strings.HasPrefix(trimmed, "dyn(") && strings.HasSuffix(trimmed, ")") {
		trimmed = strings.TrimSpace(trimmed[4 : len(trimmed)-1])
		if trimmed == "variables" || (varAliases != nil && varAliases[trimmed]) {
			return true
		}
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
				if sel.FieldName() == varName && (isVariablesExpr(sel.Operand(), varAliases) || referencesVariablesRoot(sel.Operand(), varAliases)) {
					found = true
				}
			case celast.CallKind:
				call := e.AsCall()
				fn := call.FunctionName()
				if (fn == "_[_]" || fn == "_[?_]") && len(call.Args()) == 2 {
					if (isVariablesExpr(call.Args()[0], varAliases) || referencesVariablesRoot(call.Args()[0], varAliases)) && call.Args()[1].Kind() == celast.LiteralKind {
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
		prefixes := []string{alias + ".", alias + ".?", "dyn(" + alias + ").", "dyn(" + alias + ").?"}
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
		if strings.Contains(expr, alias+"['"+varName+"']") || strings.Contains(expr, alias+`["`+varName+`"]`) ||
			strings.Contains(expr, "dyn("+alias+")['"+varName+"']") || strings.Contains(expr, "dyn("+alias+`)["`+varName+`"]`) {
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
				if isVariablesExpr(call.Args()[0], varAliases) && call.Args()[1].Kind() != celast.LiteralKind {
					hasDynamic = true
				}
			}
		}))
		if hasDynamic {
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
		if strings.Contains(expr, "dyn("+alias+")[") ||
			(strings.Contains(expr, alias+"[") && !strings.Contains(expr, alias+"['") && !strings.Contains(expr, alias+`["`)) {
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
	if len(l.secretVars) > 0 {
		if hasDynamicVariableAccess(expr, l.varAliases) || isVariablesAlias(expr, l.varAliases) {
			return true
		}
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
func (l logFormat) breakdown(e *LogExpression, et ExpressionTrace, isSec bool) {
	e.LoopValuesOmitted = et.LoopValuesOmitted
	isSecExpr := l.secret || isSec || l.isSecretExpr(et.Source)
	for _, n := range et.Nodes {
		valExpr := n.Expression
		errExpr := n.Expression
		if isSecExpr {
			valExpr = et.Source
			errExpr = et.Source
		}
		val := l.value(n.Value, valExpr)
		err := l.error(n.Error, errExpr)
		if isSecExpr {
			if val != "" && val != "true" && val != "false" {
				val = redacted
			}
			if err != "" {
				err = redacted
			}
		}
		e.Evaluated = append(e.Evaluated, LogNode{
			Expression: n.Expression,
			Value:      val,
			Error:      err,
		})
	}
}
