package trace

import (
	"slices"
	"strings"

	celast "cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
)

// Inputs says what an expression reads, as far as hiding Secret data needs to know. Build fills
// it in from the expression's AST; a trace built by hand leaves it empty.
//
// A value does not say where it came from, so whether it may hold Secret data is decided from the
// policy text instead: an expression that reads a Secret through the resource library, or reads a
// variable that does, is hidden (see redaction). Where the policy does not say which resource it
// reads, it is taken to be a Secret, so a value is hidden whenever it may be Secret data.
type Inputs struct {
	// Secret is set when the expression may read a Secret itself: it calls resource.Get, List or
	// Post for secrets, or for a resource that is not written out as a literal.
	Secret bool
	// Variables are the names of the variables the expression reads.
	Variables []string
	// AnyVariable is set when the expression reads variables without naming them, as
	// variables[object.metadata.name] or the whole variables map does, so it may read any of them.
	AnyVariable bool
}

// resourceCallPrefixes are the overload ID prefixes of the resource library functions that
// return resources: Get, List and Post (which returns the resource it created). ToGVR does not
// read anything. The same prefixes as in compiler.notRepeatableCalls, which this package cannot
// import.
var resourceCallPrefixes = []string{"resource_get_", "resource_list_", "list_resources_", "resource_post_"}

// inputsOf returns what e reads. references is the AST's reference map, which holds the overload
// IDs of a checked AST; an AST that was only parsed has none, and the resource calls are then
// recognized by their name. bound is what each name bound in the expression reads (see
// boundInputs), which a use of the name reads too.
func inputsOf(e celast.Expr, references map[int64]*celast.ReferenceInfo, bound map[string]Inputs) Inputs {
	var in Inputs
	// named holds the variables identifiers that a read by name uses, such as the one in
	// variables.password; any other use of the identifier reads variables without naming them.
	// PreOrderVisit reaches a read before its identifier, so the set is filled first.
	named := map[int64]struct{}{}
	celast.PreOrderVisit(e, celast.NewExprVisitor(func(n celast.Expr) {
		if name, variables, ok := namedVariable(n); ok {
			in.addVariable(name)
			named[variables.ID()] = struct{}{}
		}
		switch n.Kind() {
		case celast.CallKind:
			if isResourceCall(n, references[n.ID()]) && maybeSecrets(resourceArg(n, references[n.ID()])) {
				in.Secret = true
			}
		case celast.IdentKind:
			if _, ok := named[n.ID()]; !ok && n.AsIdent() == "variables" {
				in.AnyVariable = true
			}
			if b, ok := bound[n.AsIdent()]; ok {
				in.merge(b)
			}
		}
	}))
	return in
}

// boundInputs returns, for each name the expression e binds, what the value bound to it reads.
// cel.bind(name, value, body) and x.optMap(name, body) expand into a comprehension whose
// accumulator is the name and whose initial value is the value, and a loop such as all() or map()
// binds its variable to the items of the list it loops over. A node that uses the name reads what
// that value reads, though the name alone does not show it: pw in cel.bind(pw, variables.password,
// pw == ”) reads the password. Names are matched by name, not by scope, so a name that two
// comprehensions bind reads what either value reads: that can hide a value that is not Secret
// data, never show one that is.
func boundInputs(e celast.Expr, references map[int64]*celast.ReferenceInfo) map[string]Inputs {
	var comprehensions []celast.ComprehensionExpr
	celast.PreOrderVisit(e, celast.NewExprVisitor(func(n celast.Expr) {
		if n.Kind() == celast.ComprehensionKind {
			comprehensions = append(comprehensions, n.AsComprehension())
		}
	}))
	bound := map[string]Inputs{}
	// a value can use a name another comprehension binds, so this repeats until nothing changes
	for changed := true; changed; {
		changed = false
		bind := func(name string, values ...celast.Expr) {
			in := bound[name]
			for _, value := range values {
				if in.merge(inputsOf(value, references, bound)) {
					changed = true
				}
			}
			bound[name] = in
		}
		for _, c := range comprehensions {
			bind(c.AccuVar(), c.AccuInit(), c.IterRange())
			bind(c.IterVar(), c.IterRange())
			if c.HasIterVar2() {
				bind(c.IterVar2(), c.IterRange())
			}
		}
	}
	return bound
}

// merge adds what other reads to in, and reports whether that changed in.
func (in *Inputs) merge(other Inputs) bool {
	changed := false
	if other.Secret && !in.Secret {
		in.Secret, changed = true, true
	}
	if other.AnyVariable && !in.AnyVariable {
		in.AnyVariable, changed = true, true
	}
	for _, name := range other.Variables {
		if !slices.Contains(in.Variables, name) {
			in.Variables = append(in.Variables, name)
			changed = true
		}
	}
	return changed
}

func (in *Inputs) addVariable(name string) {
	if !slices.Contains(in.Variables, name) {
		in.Variables = append(in.Variables, name)
	}
}

// namedVariable returns the variable e reads by name, as variables.name, variables.?name,
// variables['name'] or variables[?'name'], and the variables identifier it reads it from.
func namedVariable(e celast.Expr) (string, celast.Expr, bool) {
	var operand celast.Expr
	var name string
	switch e.Kind() {
	case celast.SelectKind:
		operand, name = e.AsSelect().Operand(), e.AsSelect().FieldName()
	case celast.CallKind:
		call := e.AsCall()
		switch call.FunctionName() {
		case "_[_]", "_[?_]", "_?._":
		default:
			return "", nil, false
		}
		if len(call.Args()) != 2 {
			return "", nil, false
		}
		literal, ok := stringLiteral(call.Args()[1])
		if !ok {
			return "", nil, false
		}
		operand, name = call.Args()[0], literal
	default:
		return "", nil, false
	}
	if operand.Kind() != celast.IdentKind || operand.AsIdent() != "variables" {
		return "", nil, false
	}
	return name, operand, true
}

// isResourceCall reports whether e calls resource.Get, List or Post.
func isResourceCall(e celast.Expr, reference *celast.ReferenceInfo) bool {
	if reference != nil && len(reference.OverloadIDs) > 0 {
		for _, overload := range reference.OverloadIDs {
			for _, prefix := range resourceCallPrefixes {
				if strings.HasPrefix(overload, prefix) {
					return true
				}
			}
		}
		return false
	}
	call := e.AsCall()
	if !call.IsMemberFunction() || call.Target().Kind() != celast.IdentKind || call.Target().AsIdent() != "resource" {
		return false
	}
	switch call.FunctionName() {
	case "Get", "get", "List", "list", "Post", "post":
		return true
	}
	return false
}

// resourceArg returns the argument of a resource call that names the resource it reads, or nil
// when the call does not name it directly. The calls that take strings name it second, after the
// apiVersion: Get('v1', 'secrets', ...). The ones that take a GVR name it in the GVR, which is
// readable here only when it is built in place with resource.ToGVR('v1', 'secrets').
func resourceArg(e celast.Expr, reference *celast.ReferenceInfo) celast.Expr {
	args := e.AsCall().Args()
	if len(args) == 0 {
		return nil
	}
	gvr := args[0].Kind() == celast.CallKind
	if reference != nil && len(reference.OverloadIDs) > 0 {
		gvr = slices.ContainsFunc(reference.OverloadIDs, func(overload string) bool { return strings.Contains(overload, "gvr") })
	}
	if !gvr {
		if len(args) < 2 {
			return nil
		}
		return args[1]
	}
	if args[0].Kind() != celast.CallKind {
		return nil
	}
	toGVR := args[0].AsCall()
	if toGVR.FunctionName() != "ToGVR" && toGVR.FunctionName() != "toGVR" || len(toGVR.Args()) != 2 {
		return nil
	}
	return toGVR.Args()[1]
}

// maybeSecrets reports whether a resource argument may name secrets: when it is the literal
// "secrets", and when it is not a literal at all, since its value is then unknown.
func maybeSecrets(arg celast.Expr) bool {
	name, ok := stringLiteral(arg)
	return !ok || strings.EqualFold(name, "secrets")
}

func stringLiteral(e celast.Expr) (string, bool) {
	if e == nil || e.Kind() != celast.LiteralKind {
		return "", false
	}
	s, ok := e.AsLiteral().(types.String)
	return string(s), ok
}

// redaction decides which values of a decision are shown and which are hidden because they may
// be Secret data. Render and LogValues both use it, so the CLI and the admission log hide the
// same values.
type redaction struct {
	// all is set when the resource itself is a Secret: any value read from it may be its data.
	all bool
	// variables are the variables whose value may be Secret data.
	variables map[string]struct{}
	// any is set when anything in the decision may read a Secret.
	any bool
}

func newRedaction(d *Decision) redaction {
	r := redaction{all: d.ResourceKind == "Secret", variables: map[string]struct{}{}}
	// a variable may read another one, so this repeats until no further variable is found
	for found := true; found; {
		found = false
		for _, v := range d.Variables {
			if _, ok := r.variables[v.Name]; !ok && r.reads(v.Inputs) {
				r.variables[v.Name] = struct{}{}
				found = true
			}
		}
	}
	r.any = r.all || len(r.variables) > 0 || slices.ContainsFunc(expressions(d), func(et ExpressionTrace) bool {
		return r.reads(et.Inputs)
	})
	return r
}

// reads reports whether an expression with these inputs may read a Secret.
func (r redaction) reads(in Inputs) bool {
	if r.all || in.Secret || in.AnyVariable && len(r.variables) > 0 {
		return true
	}
	for _, name := range in.Variables {
		if _, ok := r.variables[name]; ok {
			return true
		}
	}
	return false
}

// value returns v, or redacted when it may be Secret data. True and false say nothing about the
// data and are kept, so the trace still shows which checks held.
func (r redaction) value(in Inputs, v string) string {
	if v != "" && v != "true" && v != "false" && r.reads(in) {
		return redacted
	}
	return v
}

// errorText returns an expression's error, or redacted when the expression may read a Secret: an
// error can quote the value it failed on, such as invalid RFC 3339 timestamp "<value>".
func (r redaction) errorText(in Inputs, err string) string {
	if err != "" && r.reads(in) {
		return redacted
	}
	return err
}

// message returns the verdict's message. The message of an ERROR verdict is the error that ended
// the evaluation, which can quote a value as errorText says, and may come from any expression of
// the decision, so it is hidden when anything in the decision may read a Secret. Any other
// message is the policy's own, which is returned to the requester anyway, and is kept.
func (r redaction) message(v VerdictTrace) string {
	if v.Status == VerdictError && v.Message != "" && r.any {
		return redacted
	}
	return v.Message
}

// expressions returns every expression traced in d.
func expressions(d *Decision) []ExpressionTrace {
	all := make([]ExpressionTrace, 0, 1+len(d.Match)+len(d.Variables)+len(d.Validations)+len(d.Mutations)+len(d.Generations))
	all = append(all, d.Verdict.ExpressionTrace)
	for _, m := range d.Match {
		all = append(all, m.ExpressionTrace)
	}
	for _, v := range d.Variables {
		all = append(all, v.ExpressionTrace)
	}
	for _, v := range d.Validations {
		all = append(all, v.ExpressionTrace)
	}
	for _, m := range d.Mutations {
		all = append(all, m.ExpressionTrace)
	}
	for _, g := range d.Generations {
		all = append(all, g.ExpressionTrace)
	}
	return all
}

// decision returns a copy of d with every value and error that may be Secret data replaced, as
// Render and LogValues show them. d itself is not changed.
func (r redaction) decision(d *Decision) *Decision {
	shown := *d
	shown.Match = r.named(d.Match)
	shown.Variables = r.named(d.Variables)
	if d.Validations != nil {
		shown.Validations = make([]ValidationTrace, len(d.Validations))
		for i, v := range d.Validations {
			v.ExpressionTrace = r.expression(v.ExpressionTrace)
			shown.Validations[i] = v
		}
	}
	shown.Verdict.ExpressionTrace = r.expression(d.Verdict.ExpressionTrace)
	shown.Verdict.Message = r.message(d.Verdict)
	if d.Mutations != nil {
		shown.Mutations = make([]MutationTrace, len(d.Mutations))
		for i, m := range d.Mutations {
			m.ExpressionTrace = r.expression(m.ExpressionTrace)
			m.Error = r.errorText(m.Inputs, m.Error)
			shown.Mutations[i] = m
		}
	}
	if d.Generations != nil {
		shown.Generations = make([]GenerationTrace, len(d.Generations))
		for i, g := range d.Generations {
			g.ExpressionTrace = r.expression(g.ExpressionTrace)
			g.Error = r.errorText(g.Inputs, g.Error)
			shown.Generations[i] = g
		}
	}
	return &shown
}

func (r redaction) named(traces []NamedExpressionTrace) []NamedExpressionTrace {
	if traces == nil {
		return nil
	}
	shown := make([]NamedExpressionTrace, len(traces))
	for i, n := range traces {
		n.ExpressionTrace = r.expression(n.ExpressionTrace)
		shown[i] = n
	}
	return shown
}

func (r redaction) expression(et ExpressionTrace) ExpressionTrace {
	et.Result = r.value(et.Inputs, et.Result)
	if et.Nodes != nil {
		nodes := make([]NodeTrace, len(et.Nodes))
		for i, n := range et.Nodes {
			n.Value = r.value(n.Inputs, n.Value)
			n.Error = r.errorText(n.Inputs, n.Error)
			nodes[i] = n
		}
		et.Nodes = nodes
	}
	return et
}
