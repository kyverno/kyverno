package engine

import (
	"fmt"
	"maps"
	"slices"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/interpreter"
	"cel.dev/cel-go/parser"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"k8s.io/apiserver/pkg/cel/lazy"
)

// residual returns a self-contained CEL condition over `object` and
// `oldObject` for a rule whose partial evaluation is unknown.
//
// Known values are first substituted by pruning the checked AST with the
// evaluation state (cel-go ResidualAst). Pruning does not descend into
// comprehension bodies, and some values cannot be written as literals, so any
// remaining reference to `request` or `variables` is captured with
// `cel.bind(request, <value>, ...)` and `cel.bind(variables, <value>, ...)`.
// The result therefore never depends on later policy edits. If pruning fails,
// the source expression is captured the same way, which evaluates exactly as
// the full expression would.
func (e *engine) residual(ev *evaluation, pol *apolcompiler.Policy, rule apolcompiler.CompiledRule, details *cel.EvalDetails, vars *lazy.MapValue) (string, error) {
	expr, src, err := pruned(rule.AST, details)
	if err != nil {
		logger.V(4).Info("residual pruning failed, capturing the source expression", "policy", pol.Policy.Name, "rule", rule.Name, "error", err)
		expr, src = rule.AST.NativeRep().Expr(), rule.Expression
	}
	refs := freeReferences(expr)
	builder := newLiteralBuilder(ev.limits.MaxValueDepth)
	if r, ok := refs[celcompiler.VariablesKey]; ok {
		val, err := captureVariables(pol, vars, r)
		if err != nil {
			return "", err
		}
		lit, err := builder.render(val)
		if err != nil {
			return "", fmt.Errorf("capturing variables: %w", err)
		}
		src = fmt.Sprintf("cel.bind(%s, %s, %s)", celcompiler.VariablesKey, lit, src)
	}
	if r, ok := refs[celcompiler.RequestKey]; ok {
		val := captureRequest(ev.request, r)
		lit, err := builder.render(types.DefaultTypeAdapter.NativeToValue(val))
		if err != nil {
			return "", fmt.Errorf("capturing request: %w", err)
		}
		src = fmt.Sprintf("cel.bind(%s, %s, %s)", celcompiler.RequestKey, lit, src)
	}
	if len(src) > ev.limits.MaxConditionBytes {
		return "", fmt.Errorf("residual condition is %d bytes, more than the %d byte limit", len(src), ev.limits.MaxConditionBytes)
	}
	// The residual must compile in the conditions environment, which declares
	// only object and oldObject: it is self-contained. Compiled programs are
	// cached, so a repeated residual is not parsed again on the request path.
	if _, err := e.program(src); err != nil {
		return "", fmt.Errorf("residual %q is not self-contained: %w", truncate(src, 256), err)
	}
	return src, nil
}

// pruned substitutes known values into the checked AST using the evaluation
// state (constant folding) and renders the result. It is cel-go's
// Env.ResidualAst without the re-parse and re-check: the macro call map is
// cloned because the pruner mutates it, and the compiled AST is shared by
// concurrent requests.
func pruned(checked *cel.Ast, details *cel.EvalDetails) (ast.Expr, string, error) {
	native := checked.NativeRep()
	res := interpreter.PruneAst(native.Expr(), maps.Clone(native.SourceInfo().MacroCalls()), details.State())
	src, err := parser.Unparse(res.Expr(), res.SourceInfo())
	if err != nil {
		return nil, "", err
	}
	return res.Expr(), src, nil
}

// reference describes how an expression uses a top-level identifier.
type reference struct {
	// whole is true when the identifier is used other than through a field
	// selection, so the whole value must be captured.
	whole  bool
	fields []string
}

// freeReferences returns the uses of `request` and `variables` in an
// expression. A comprehension variable with the same name is over-approximated
// as a reference, which only captures a value that is not needed.
func freeReferences(expr ast.Expr) map[string]*reference {
	idents := map[string]int{}
	selects := map[string]int{}
	out := map[string]*reference{}
	ast.PreOrderVisit(expr, ast.NewExprVisitor(func(e ast.Expr) {
		switch e.Kind() {
		case ast.IdentKind:
			name := e.AsIdent()
			if name == celcompiler.RequestKey || name == celcompiler.VariablesKey {
				idents[name]++
				if out[name] == nil {
					out[name] = &reference{}
				}
			}
		case ast.SelectKind:
			sel := e.AsSelect()
			if sel.Operand().Kind() != ast.IdentKind {
				return
			}
			name := sel.Operand().AsIdent()
			if name == celcompiler.RequestKey || name == celcompiler.VariablesKey {
				selects[name]++
				if out[name] == nil {
					out[name] = &reference{}
				}
				if !slices.Contains(out[name].fields, sel.FieldName()) {
					out[name].fields = append(out[name].fields, sel.FieldName())
				}
			}
		}
	}))
	for name, r := range out {
		if idents[name] > selects[name] {
			r.whole = true
		}
		slices.Sort(r.fields)
	}
	return out
}

// captureRequest returns the part of the request a residual uses.
func captureRequest(request map[string]any, r *reference) map[string]any {
	if r.whole {
		return request
	}
	out := make(map[string]any, len(r.fields))
	for _, f := range r.fields {
		if v, ok := request[f]; ok {
			out[f] = v
		}
	}
	return out
}

// captureVariables evaluates the variables a residual uses. A variable that
// cannot be evaluated at authorization time makes the rule an error.
func captureVariables(pol *apolcompiler.Policy, vars *lazy.MapValue, r *reference) (ref.Val, error) {
	names := r.fields
	if r.whole {
		names = make([]string, 0, len(pol.Variables))
		for _, v := range pol.Variables {
			names = append(names, v.Name)
		}
	}
	out := make(map[string]ref.Val, len(names))
	for _, name := range names {
		val := vars.Get(types.String(name))
		if types.IsUnknownOrError(val) {
			return nil, fmt.Errorf("variable %q is not available at authorization time: %v", name, val)
		}
		out[name] = val
	}
	return types.NewStringInterfaceMap(types.DefaultTypeAdapter, toInterfaceMap(out)), nil
}

func toInterfaceMap(in map[string]ref.Val) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// program returns the cached program of a condition, compiling it in the
// conditions environment on a cache miss.
func (e *engine) program(expr string) (cel.Program, error) {
	if cached, ok := e.programs.Get(expr); ok {
		if prog, ok := cached.(cel.Program); ok {
			return prog, nil
		}
	}
	e.celMu.Lock()
	checked, iss := e.conditionsEnv.Compile(expr)
	e.celMu.Unlock()
	if iss.Err() != nil {
		return nil, iss.Err()
	}
	if out := checked.OutputType(); !out.IsExactType(types.BoolType) && !out.IsExactType(types.DynType) {
		return nil, fmt.Errorf("condition must evaluate to bool, got %v", out)
	}
	prog, err := e.conditionsEnv.Program(checked, apolcompiler.ProgramOptions(false)...)
	if err != nil {
		return nil, err
	}
	e.programs.Add(expr, prog)
	return prog, nil
}
