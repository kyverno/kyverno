// Package template implements the YAML template declaration mode for
// GeneratingPolicy generate entries (spec.generate[].template).
//
// A template is a YAML string (single or multi-document) describing the
// resources to generate. When interpolation is enabled (interpolate: cel),
// `(( ... ))` placeholders are evaluated as CEL expressions against the same
// evaluation context as generation expressions (object, variables, ...):
//
//   - a placeholder occupying an entire scalar value is spliced structurally,
//     so it may evaluate to any CEL value (map, list, scalar);
//   - a placeholder embedded in a larger string is interpolated and must
//     evaluate to a scalar;
//   - placeholders are not supported in mapping keys; use a whole-value
//     placeholder on the parent field instead (e.g. `labels: (( variables.labels ))`).
package template

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Template is a compiled generation template. It is safe for concurrent use;
// Render produces a fresh resource tree on every call.
type Template struct {
	docs []node
}

// Compile parses and compiles a generation template. All placeholder CEL
// expressions are compiled eagerly so that authoring errors are reported at
// admission time with their YAML location.
func Compile(path *field.Path, env *cel.Env, tpl *policiesv1beta1.GenerationTemplate) (*Template, field.ErrorList) {
	path = path.Child("value")
	interpolate := tpl.Interpolate == policiesv1beta1.InterpolationModeCEL
	c := &templateCompiler{env: env, interpolate: interpolate}
	decoder := yaml.NewDecoder(strings.NewReader(tpl.Value))
	var docs []node
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, field.ErrorList{field.Invalid(path, tpl.Value, fmt.Sprintf("failed to parse YAML: %v", err))}
		}
		root := &doc
		if root.Kind == yaml.DocumentNode {
			if len(root.Content) == 0 {
				continue
			}
			root = root.Content[0]
		}
		if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
			continue
		}
		n, err := c.compileNode(root)
		if err != nil {
			return nil, field.ErrorList{field.Invalid(path, tpl.Value, err.Error())}
		}
		if !isMappingRoot(n) {
			return nil, field.ErrorList{field.Invalid(path, tpl.Value, fmt.Sprintf("line %d: document must be a mapping describing a resource", root.Line))}
		}
		docs = append(docs, n)
	}
	if len(docs) == 0 {
		return nil, field.ErrorList{field.Invalid(path, tpl.Value, "template must contain at least one YAML document")}
	}
	return &Template{docs: docs}, nil
}

// Render evaluates all placeholders against the given CEL activation and
// returns one resource map per YAML document.
func (t *Template) Render(ctx context.Context, activation any) ([]map[string]any, error) {
	resources := make([]map[string]any, 0, len(t.docs))
	for i, doc := range t.docs {
		v, err := doc.render(ctx, activation)
		if err != nil {
			return nil, fmt.Errorf("document %d: %w", i, err)
		}
		resource, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("document %d: expected a mapping describing a resource, got %T", i, v)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// ExtractExpressions returns the CEL expressions of all placeholders found in
// the given template value. It is best-effort: parse errors yield an empty
// result (they are reported separately by Compile).
func ExtractExpressions(value string) []string {
	var expressions []string
	decoder := yaml.NewDecoder(strings.NewReader(value))
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			break
		}
		collectExpressions(&doc, &expressions)
	}
	return expressions
}

func collectExpressions(n *yaml.Node, out *[]string) {
	if n == nil {
		return
	}
	if n.Kind == yaml.ScalarNode && containsPlaceholder(n.Value) {
		if segments, err := scan(n.Value); err == nil {
			for _, s := range segments {
				if s.isExpr {
					*out = append(*out, s.expression)
				}
			}
		}
	}
	for _, child := range n.Content {
		collectExpressions(child, out)
	}
}

// isMappingRoot reports whether a compiled document root can produce a
// resource map: a mapping, a static mapping literal, or a whole-document
// placeholder (validated at render time).
func isMappingRoot(n node) bool {
	switch n := n.(type) {
	case *mappingNode, *spliceNode:
		return true
	case *literalNode:
		_, ok := n.value.(map[string]any)
		return ok
	default:
		return false
	}
}

type templateCompiler struct {
	env         *cel.Env
	interpolate bool
}

// node is a compiled fragment of a template document.
type node interface {
	render(ctx context.Context, activation any) (any, error)
}

// literalNode holds a static subtree decoded from YAML. The value is deep
// copied on render because the generation runtime mutates resource maps.
type literalNode struct {
	value any
}

func (n *literalNode) render(context.Context, any) (any, error) {
	return deepCopyValue(n.value), nil
}

type mappingEntry struct {
	key   string
	value node
}

type mappingNode struct {
	entries []mappingEntry
}

func (n *mappingNode) render(ctx context.Context, activation any) (any, error) {
	m := make(map[string]any, len(n.entries))
	for _, e := range n.entries {
		v, err := e.value.render(ctx, activation)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", e.key, err)
		}
		m[e.key] = v
	}
	return m, nil
}

type sequenceNode struct {
	items []node
}

func (n *sequenceNode) render(ctx context.Context, activation any) (any, error) {
	s := make([]any, 0, len(n.items))
	for i, item := range n.items {
		v, err := item.render(ctx, activation)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		s = append(s, v)
	}
	return s, nil
}

// spliceNode is a scalar consisting of exactly one placeholder: the CEL result
// is spliced structurally and may be of any type (map, list, scalar).
type spliceNode struct {
	program    cel.Program
	expression string
}

func (n *spliceNode) render(ctx context.Context, activation any) (any, error) {
	out, _, err := n.program.ContextEval(ctx, activation)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate placeholder %q: %w", n.expression, err)
	}
	v, err := celValueToNative(out)
	if err != nil {
		return nil, fmt.Errorf("failed to convert placeholder %q result: %w", n.expression, err)
	}
	return v, nil
}

// interpolatedNode is a scalar mixing literal fragments and placeholders; the
// result is a string and every placeholder must evaluate to a scalar.
type interpolatedNode struct {
	segments []segment
	programs map[int]cel.Program
}

func (n *interpolatedNode) render(ctx context.Context, activation any) (any, error) {
	var sb strings.Builder
	for i, s := range n.segments {
		if !s.isExpr {
			sb.WriteString(s.literal)
			continue
		}
		out, _, err := n.programs[i].ContextEval(ctx, activation)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate placeholder %q: %w", s.expression, err)
		}
		v, err := celValueToNative(out)
		if err != nil {
			return nil, fmt.Errorf("failed to convert placeholder %q result: %w", s.expression, err)
		}
		switch v := v.(type) {
		case string:
			sb.WriteString(v)
		case bool, int64, uint64, float64:
			fmt.Fprintf(&sb, "%v", v)
		default:
			return nil, fmt.Errorf("placeholder %q embedded in a string must evaluate to a scalar, got %T; use a whole-value placeholder for structured results", s.expression, v)
		}
	}
	return sb.String(), nil
}

func (c *templateCompiler) compileNode(n *yaml.Node) (node, error) {
	if n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if !c.interpolate || !nodeContainsPlaceholder(n) {
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, fmt.Errorf("line %d: %v", n.Line, err)
		}
		return &literalNode{value: v}, nil
	}
	switch n.Kind {
	case yaml.MappingNode:
		entries := make([]mappingEntry, 0, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			if key.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("line %d: only scalar mapping keys are supported", key.Line)
			}
			if containsPlaceholder(key.Value) {
				return nil, fmt.Errorf("line %d: placeholders are not supported in mapping keys; use a whole-value placeholder on the parent field instead (e.g. `labels: (( variables.labels ))`)", key.Line)
			}
			if key.Value == "<<" {
				return nil, fmt.Errorf("line %d: YAML merge keys are not supported in subtrees containing placeholders", key.Line)
			}
			v, err := c.compileNode(value)
			if err != nil {
				return nil, err
			}
			entries = append(entries, mappingEntry{key: key.Value, value: v})
		}
		return &mappingNode{entries: entries}, nil
	case yaml.SequenceNode:
		items := make([]node, 0, len(n.Content))
		for _, item := range n.Content {
			v, err := c.compileNode(item)
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		}
		return &sequenceNode{items: items}, nil
	case yaml.ScalarNode:
		return c.compileScalar(n)
	default:
		return nil, fmt.Errorf("line %d: unsupported YAML node kind", n.Line)
	}
}

func (c *templateCompiler) compileScalar(n *yaml.Node) (node, error) {
	segments, err := scan(n.Value)
	if err != nil {
		return nil, fmt.Errorf("line %d: %v", n.Line, err)
	}
	expressions := 0
	for _, s := range segments {
		if s.isExpr {
			expressions++
		}
	}
	if expressions == 0 {
		// only escape sequences, no placeholder to evaluate
		var sb strings.Builder
		for _, s := range segments {
			sb.WriteString(s.literal)
		}
		return &literalNode{value: sb.String()}, nil
	}
	if expressions == 1 && len(segments) == 1 {
		program, err := c.compileExpression(n, segments[0].expression)
		if err != nil {
			return nil, err
		}
		return &spliceNode{program: program, expression: segments[0].expression}, nil
	}
	programs := make(map[int]cel.Program, expressions)
	for i, s := range segments {
		if !s.isExpr {
			continue
		}
		program, err := c.compileExpression(n, s.expression)
		if err != nil {
			return nil, err
		}
		programs[i] = program
	}
	return &interpolatedNode{segments: segments, programs: programs}, nil
}

func (c *templateCompiler) compileExpression(n *yaml.Node, expression string) (cel.Program, error) {
	ast, issues := c.env.Compile(expression)
	if err := issues.Err(); err != nil {
		return nil, fmt.Errorf("line %d: invalid placeholder expression %q: %v", n.Line, expression, err)
	}
	program, err := c.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("line %d: failed to build placeholder program %q: %v", n.Line, expression, err)
	}
	return program, nil
}

func nodeContainsPlaceholder(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.AliasNode {
		return nodeContainsPlaceholder(n.Alias)
	}
	if n.Kind == yaml.ScalarNode && (containsPlaceholder(n.Value) || containsEscape(n.Value)) {
		return true
	}
	for _, child := range n.Content {
		if nodeContainsPlaceholder(child) {
			return true
		}
	}
	return false
}

// celValueToNative converts a CEL evaluation result into a plain Go value
// suitable for an unstructured resource map.
func celValueToNative(val ref.Val) (any, error) {
	if types.IsError(val) {
		return nil, fmt.Errorf("%v", val)
	}
	switch val.Type() {
	case types.NullType:
		return nil, nil
	case types.StringType:
		return val.Value().(string), nil
	case types.BoolType:
		return val.Value().(bool), nil
	case types.IntType:
		return val.Value().(int64), nil
	case types.UintType:
		return int64(val.Value().(uint64)), nil //nolint:gosec
	case types.DoubleType:
		return val.Value().(float64), nil
	}
	pv, err := val.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
	if err != nil {
		return nil, err
	}
	return pv.(*structpb.Value).AsInterface(), nil
}

// deepCopyValue deep copies YAML-decoded values (maps, slices, scalars).
func deepCopyValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = deepCopyValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, deepCopyValue(item))
		}
		return out
	default:
		return v
	}
}
