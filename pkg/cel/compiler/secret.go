package compiler

import (
	"strings"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
)

// MayReadSecret reports whether ast reads a resource kind that is or may be a Secret.
func MayReadSecret(ast *cel.Ast) bool {
	if ast == nil {
		return false
	}
	found := false
	native := ast.NativeRep()
	celast.PreOrderVisit(native.Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if found || e.Kind() != celast.CallKind {
			return
		}
		call := e.AsCall()
		if !call.IsMemberFunction() {
			return
		}
		isGet, isList := isResourceRead(call, native.GetOverloadIDs(e.ID()))
		if !isGet && !isList {
			return
		}
		found = resourceCallMayReadSecret(call)
	}))
	return found
}

func isResourceRead(call celast.CallExpr, overloadIDs []string) (bool, bool) {
	var isGet, isList bool
	for _, overloadID := range overloadIDs {
		overloadID = strings.ToLower(overloadID)
		isGet = isGet || strings.HasPrefix(overloadID, "resource_get_") || strings.HasPrefix(overloadID, "get_resources_gvr_")
		isList = isList || strings.HasPrefix(overloadID, "resource_list_") || strings.HasPrefix(overloadID, "list_resources_gvr_")
	}
	if isGet || isList {
		return isGet, isList
	}
	if call.Target().Kind() != celast.IdentKind || call.Target().AsIdent() != "resource" {
		return false, false
	}
	function := strings.ToLower(call.FunctionName())
	return function == "get" || strings.HasPrefix(function, "resource_get_"),
		function == "list" || strings.HasPrefix(function, "resource_list_")
}

func resourceCallMayReadSecret(call celast.CallExpr) bool {
	args := call.Args()
	if len(args) == 0 {
		return true
	}
	var resourceArg []celast.Expr
	if args[0].Kind() == celast.CallKind {
		toGVR := args[0].AsCall()
		name := strings.ToLower(toGVR.FunctionName())
		if strings.Contains(name, "togvr") || strings.Contains(name, "to_gvr") {
			toGVRArgs := toGVR.Args()
			if len(toGVRArgs) < 2 {
				return true
			}
			resourceArg = toGVRArgs[1:]
		}
	}
	if resourceArg == nil {
		if args[0].Kind() != celast.LiteralKind || len(args) < 2 {
			return true
		}
		resourceArg = args[1:]
	}
	if len(resourceArg) == 0 || resourceArg[0].Kind() != celast.LiteralKind {
		return true
	}
	name, ok := resourceArg[0].AsLiteral().Value().(string)
	if !ok {
		return true
	}
	name = strings.ToLower(name)
	return name == "secret" || name == "secrets"
}
