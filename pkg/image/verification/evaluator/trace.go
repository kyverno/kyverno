package evaluator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/trace"
)

// callsImageVerification reports whether the expression calls one of the image verification
// functions (verifyImageSignatures, verifyAttestationSignatures, getImageData, extractPayload).
// Their macros rewrite every such call into a member call on the imageverify runtime, so that is
// what is looked for.
//
// Those calls reach out to registries and record into the request-wide verification results that
// validationConfigurations.required reads, so an expression making them must never be re-run to
// explain it: the re-run would repeat the network calls, could get a different answer than the
// one that decided, and could record a verification the decision never made.
func callsImageVerification(ast *cel.Ast) bool {
	if ast == nil {
		return false
	}
	found := false
	celast.PreOrderVisit(ast.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if e.Kind() != celast.CallKind {
			return
		}
		if call := e.AsCall(); call.IsMemberFunction() && call.Target().Kind() == celast.IdentKind && call.Target().AsIdent() == imageverify.RuntimeKey {
			found = true
		}
	}))
	return found
}

// withoutVerificationTwin drops the explain-only tracking program of an expression that calls
// image verification (see callsImageVerification), keeping its AST so the trace still shows the
// expression and its result. With no twin, compiler.TraceDetails has nothing to re-run.
func withoutVerificationTwin(traced engine.TracedProgram) engine.TracedProgram {
	if callsImageVerification(traced.AST) {
		traced.Traced = nil
	}
	return traced
}

// buildExpressionTrace turns one traced evaluation into an ExpressionTrace. The source text is
// read back from the retained AST. When the evaluation failed outright and produced no value, the
// error itself becomes the result so the trace shows why. Mirrors vpol's buildExpressionTrace
// (pkg/cel/policies/vpol/compiler/policy.go).
func buildExpressionTrace(ast *cel.Ast, out ref.Val, details *cel.EvalDetails, err error) trace.ExpressionTrace {
	if out == nil && err != nil {
		out = types.WrapErr(err)
	}
	source := ""
	if ast != nil {
		source = ast.Source().Content()
	}
	et := trace.Build(source, ast, out, details)
	if callsImageVerification(ast) {
		et.NoBreakdown = "it verifies images, and verification is never run a second time"
	}
	return et
}

// skipMessage says why the match conditions skipped the policy: either one came out false (match
// stops there), or none did and some errored, which failurePolicy Ignore treats as a non-match.
// Mirrors vpol's skipMessage (pkg/cel/policies/vpol/compiler/policy.go).
func skipMessage(recorded int, excludedBy string, errored []string) string {
	switch {
	case recorded == 0:
		return "a match condition excluded this resource"
	case excludedBy != "":
		return fmt.Sprintf("match condition %q did not pass, so the policy was skipped", excludedBy)
	case len(errored) == 1:
		return fmt.Sprintf("match condition %q failed to evaluate and failurePolicy is Ignore, so the policy was skipped", errored[0])
	case len(errored) > 1:
		quoted := make([]string, 0, len(errored))
		for _, name := range errored {
			quoted = append(quoted, strconv.Quote(name))
		}
		return fmt.Sprintf("match conditions %s failed to evaluate and failurePolicy is Ignore, so the policy was skipped", strings.Join(quoted, ", "))
	}
	return "a match condition did not pass, so the policy was skipped"
}

// exemptMessage names the exceptions that exempted the resource, as the engine's rule message
// does.
func exemptMessage(exceptions []*policiesv1beta1.PolicyException) string {
	names := make([]string, 0, len(exceptions))
	for _, polex := range exceptions {
		names = append(names, polex.GetNamespace()+"/"+polex.GetName())
	}
	return "rule is skipped due to policy exception: " + strings.Join(names, ", ")
}
