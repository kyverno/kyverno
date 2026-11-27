package engine

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/parser"
)

// literalBuilder renders known CEL values as CEL source so they can be
// captured in a residual condition with `cel.bind`. Every list element and map
// value is wrapped in `dyn(...)`, so heterogeneous JSON values type-check.
type literalBuilder struct {
	factory  ast.ExprFactory
	nextID   int64
	maxDepth int
}

func newLiteralBuilder(maxDepth int) *literalBuilder {
	return &literalBuilder{factory: ast.NewExprFactory(), maxDepth: maxDepth}
}

func (b *literalBuilder) id() int64 {
	b.nextID++
	return b.nextID
}

// render returns the CEL source of a value.
func (b *literalBuilder) render(val ref.Val) (string, error) {
	expr, err := b.build(val, 0)
	if err != nil {
		return "", err
	}
	return parser.Unparse(expr, ast.NewSourceInfo(nil))
}

func (b *literalBuilder) dyn(val ref.Val, depth int) (ast.Expr, error) {
	expr, err := b.build(val, depth)
	if err != nil {
		return nil, err
	}
	return b.factory.NewCall(b.id(), "dyn", expr), nil
}

func (b *literalBuilder) build(val ref.Val, depth int) (ast.Expr, error) {
	if depth > b.maxDepth {
		return nil, fmt.Errorf("value nesting exceeds %d levels", b.maxDepth)
	}
	if types.IsUnknownOrError(val) {
		return nil, fmt.Errorf("value is not known: %v", val)
	}
	switch v := val.(type) {
	case types.Double:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return b.factory.NewCall(b.id(), "double", b.factory.NewLiteral(b.id(), types.String(fmt.Sprint(f)))), nil
		}
		return b.factory.NewLiteral(b.id(), v), nil
	case types.Bool, types.Bytes, types.Int, types.Null, types.String, types.Uint:
		return b.factory.NewLiteral(b.id(), v), nil
	case types.Duration:
		return b.factory.NewCall(b.id(), "duration", b.factory.NewLiteral(b.id(), v.ConvertToType(types.StringType))), nil
	case types.Timestamp:
		return b.factory.NewCall(b.id(), "timestamp", b.factory.NewLiteral(b.id(), v.ConvertToType(types.StringType))), nil
	}
	if mapper, ok := val.(traits.Mapper); ok {
		keys, err := mapKeys(mapper)
		if err != nil {
			return nil, err
		}
		entries := make([]ast.EntryExpr, 0, len(keys))
		for _, key := range keys {
			value, found := mapper.Find(key)
			if !found {
				return nil, fmt.Errorf("map key %v vanished during rendering", key)
			}
			k, err := b.build(key, depth+1)
			if err != nil {
				return nil, err
			}
			v, err := b.dyn(value, depth+1)
			if err != nil {
				return nil, fmt.Errorf("key %v: %w", key, err)
			}
			entries = append(entries, b.factory.NewMapEntry(b.id(), k, v, false))
		}
		return b.factory.NewMap(b.id(), entries), nil
	}
	if lister, ok := val.(traits.Lister); ok {
		size, ok := lister.Size().(types.Int)
		if !ok {
			return nil, fmt.Errorf("list has no size")
		}
		elems := make([]ast.Expr, 0, int(size))
		for i := types.Int(0); i < size; i++ {
			elem, err := b.dyn(lister.Get(i), depth+1)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			elems = append(elems, elem)
		}
		return b.factory.NewList(b.id(), elems, nil), nil
	}
	return nil, fmt.Errorf("values of type %s cannot be captured in a condition", val.Type().TypeName())
}

// mapKeys returns the keys of a map sorted for deterministic output.
func mapKeys(mapper traits.Mapper) ([]ref.Val, error) {
	var keys []ref.Val
	it := mapper.Iterator()
	for it.HasNext() == types.True {
		key := it.Next()
		switch key.(type) {
		case types.String, types.Int, types.Uint, types.Bool:
		default:
			return nil, fmt.Errorf("map key of type %s cannot be captured in a condition", key.Type().TypeName())
		}
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b ref.Val) int {
		return strings.Compare(fmt.Sprintf("%T:%v", a, a.Value()), fmt.Sprintf("%T:%v", b, b.Value()))
	})
	return keys, nil
}
