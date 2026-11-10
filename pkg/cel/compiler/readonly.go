package compiler

import (
	"fmt"

	celcommon "github.com/google/cel-go/common"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/parser"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var writeFunctions = map[string]map[string]struct{}{
	"resource": {"post": {}, "Post": {}},
	"http":     {"post": {}, "Post": {}},
}

// turn an expression into an ast and walk its tree to check if it contains
// one of the functions defined in writeFunctions
func FindWriteCall(expression string) string {
	parsed, iss := parser.Parse(celcommon.NewStringSource(expression, ""))
	if iss != nil && len(iss.GetErrors()) > 0 {
		return ""
	}
	found := ""
	celast.PreOrderVisit(parsed.Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if found != "" || e == nil || e.Kind() != celast.CallKind {
			return
		}
		call := e.AsCall()
		if !call.IsMemberFunction() {
			return
		}
		root := call.Target()
		for root.Kind() == celast.CallKind && root.AsCall().IsMemberFunction() {
			root = root.AsCall().Target()
		}
		if root.Kind() != celast.IdentKind {
			return
		}
		target := root.AsIdent()
		if _, ok := writeFunctions[target][call.FunctionName()]; ok {
			found = target + "." + call.FunctionName()
		}
	}))
	return found
}

func WriteCallError(path *field.Path, call string) *field.Error {
	return field.Forbidden(path, fmt.Sprintf("%s is only allowed in validations, mutations or generate expressions", call))
}

func ReadOnlyVariableErrors(path *field.Path, variables ...admissionregistrationv1.Variable) field.ErrorList {
	var errs field.ErrorList
	for i, v := range variables {
		if call := FindWriteCall(v.Expression); call != "" {
			errs = append(errs, WriteCallError(path.Index(i).Child("expression"), call))
		}
	}
	return errs
}

func ReadOnlyMatchConditionErrors(path *field.Path, conditions ...admissionregistrationv1.MatchCondition) field.ErrorList {
	var errs field.ErrorList
	for i, c := range conditions {
		if call := FindWriteCall(c.Expression); call != "" {
			errs = append(errs, WriteCallError(path.Index(i).Child("expression"), call))
		}
	}
	return errs
}
