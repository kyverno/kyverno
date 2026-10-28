package compiler

import (
	"maps"
	"slices"
	"strconv"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
)

func ImageResourcePaths(expression *cel.Ast) [][]string {
	if expression == nil || !expression.IsChecked() {
		return nil
	}
	native := expression.NativeRep()
	const (
		valuePath = iota
		iterationPath
		selectionPath
	)
	extend := func(paths [][]string, segment string) [][]string {
		if len(paths) == 0 {
			return nil
		}
		extended := make([][]string, 0, len(paths))
		for _, path := range paths {
			extended = append(extended, append(slices.Clone(path), segment))
		}
		return extended
	}
	var collect func(celast.Expr, map[string][][]string, int) [][]string
	collect = func(node celast.Expr, bindings map[string][][]string, mode int) [][]string {
		var paths [][]string
		switch node.Kind() {
		case celast.IdentKind:
			identifier := node.AsIdent()
			if bound, found := bindings[identifier]; found {
				paths = slices.Clone(bound)
			} else if identifier == "object" || identifier == "oldObject" || identifier == "variables" {
				paths = [][]string{{identifier}}
			}
		case celast.SelectKind:
			selection := node.AsSelect()
			if !selection.IsTestOnly() {
				paths = extend(collect(selection.Operand(), bindings, selectionPath), selection.FieldName())
			}
		case celast.ListKind:
			if mode == selectionPath {
				return nil
			}
			for _, element := range node.AsList().Elements() {
				paths = append(paths, collect(element, bindings, valuePath)...)
			}
			return paths
		case celast.CallKind:
			call := node.AsCall()
			arguments := call.Args()
			switch call.FunctionName() {
			case operators.Conditional:
				return append(collect(arguments[1], bindings, mode), collect(arguments[2], bindings, mode)...)
			case operators.Add:
				if mode != selectionPath && native.GetType(node.ID()).Kind() == types.ListKind {
					return append(collect(arguments[0], bindings, valuePath), collect(arguments[1], bindings, valuePath)...)
				}
			case "orValue":
				if call.IsMemberFunction() && len(arguments) == 1 {
					return append(collect(call.Target(), bindings, mode), collect(arguments[0], bindings, mode)...)
				}
			case operators.Index, operators.OptIndex, operators.OptSelect:
				if len(arguments) != 2 || arguments[1].Kind() != celast.LiteralKind {
					return nil
				}
				var segment string
				switch value := arguments[1].AsLiteral().(type) {
				case types.String:
					segment = string(value)
				case types.Int:
					segment = strconv.FormatInt(int64(value), 10)
				case types.Uint:
					segment = strconv.FormatUint(uint64(value), 10)
				default:
					return nil
				}
				paths = extend(collect(arguments[0], bindings, selectionPath), segment)
			}
		case celast.ComprehensionKind:
			comprehension := node.AsComprehension()
			if mode == selectionPath || native.GetType(node.ID()).Kind() != types.ListKind || comprehension.HasIterVar2() ||
				native.GetType(comprehension.IterRange().ID()).Kind() == types.MapKind {
				return nil
			}
			scope := maps.Clone(bindings)
			scope[comprehension.IterVar()] = collect(comprehension.IterRange(), bindings, iterationPath)
			scope[comprehension.AccuVar()] = collect(comprehension.AccuInit(), bindings, valuePath)
			scope[comprehension.AccuVar()] = collect(comprehension.LoopStep(), scope, valuePath)
			return collect(comprehension.Result(), scope, valuePath)
		}
		if mode == iterationPath {
			return extend(paths, "*")
		}
		return paths
	}
	return collect(native.Expr(), map[string][][]string{}, valuePath)
}
