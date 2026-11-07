package template

import (
	"context"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// FuzzGenerationTemplateScanner checks scanner invariants independently of
// YAML parsing and CEL compilation, keeping this target inexpensive.
func FuzzGenerationTemplateScanner(f *testing.F) {
	for _, seed := range []string{
		"",
		"plain text",
		"(( variables.name ))",
		"prefix-(( variables.name ))-suffix",
		"(( a ))-(( b ))",
		"(( string(f(x)) ))",
		`(( ":)".size() ))`,
		`\(( escaped ))`,
		`\((-(( variables.name ))`,
		"(( ))",
		"(( missing",
		`(( "unterminated ))`,
		"(( x ) unexpected ))",
		"☃-(( variables.name ))",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			t.Skip()
		}

		segments, err := scan(input)
		hasPlaceholder := containsPlaceholder(input)
		if !hasPlaceholder && err != nil {
			t.Fatalf("input without unescaped placeholders failed: %v", err)
		}
		if err != nil {
			return // malformed placeholder syntax is an expected result
		}

		var literal strings.Builder
		hasExpression := false
		for _, segment := range segments {
			if segment.isExpr {
				hasExpression = true
				if !hasPlaceholder {
					t.Fatal("extracted expression without an unescaped opener")
				}
				if strings.TrimSpace(segment.expression) == "" {
					t.Fatal("successful scan returned an empty expression")
				}
			} else {
				if segment.literal == "" {
					t.Fatal("successful scan returned an empty literal segment")
				}
				literal.WriteString(segment.literal)
			}
		}
		if hasPlaceholder && !hasExpression {
			t.Fatal("successful scan returned no expression for an unescaped opener")
		}

		// When there are no placeholders, scanning must only unescape \((.
		if !hasPlaceholder {
			want := strings.ReplaceAll(input, `\((`, "((")
			if got := literal.String(); got != want {
				t.Fatalf("literal scan mismatch: got %q, want %q", got, want)
			}
		}
	})
}

// FuzzGenerationTemplateCompileRender exercises YAML decoding, extraction,
// CEL compilation, rendering and fresh-tree isolation without cluster access.
func FuzzGenerationTemplateCompileRender(f *testing.F) {
	env, err := cel.NewEnv(
		cel.Variable("variables", cel.DynType),
		cel.Variable("object", cel.DynType),
	)
	if err != nil {
		f.Fatalf("create CEL environment: %v", err)
	}

	seeds := []struct {
		value       string
		interpolate bool
	}{
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: static
  labels:
    app: demo
`, false},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.name ))
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: prefix-(( variables.name ))-suffix
  labels: (( variables.labels ))
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: static
  finalizers: (( variables.names ))
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  annotations:
    escaped: \(( literal ))
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: first
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.name ))
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata: &meta
  name: cm
data:
  copy: *meta
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: (( ))
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables. ))
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  (( variables.name )): invalid-key
`, true},
		{`kind: [not-valid-yaml`, true},
		{`---
null
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: valid
`, true},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.name ))
`, false},
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: nested-nan
  annotations:
    value: .nan
`, false},
	}
	for _, seed := range seeds {
		f.Add(seed.value, seed.interpolate)
	}

	f.Fuzz(func(t *testing.T, value string, interpolate bool) {
		if len(value) > 4096 {
			t.Skip()
		}

		// Exercise the standalone expression-extraction path as well.
		_ = ExtractExpressions(value)

		mode := policiesv1beta1.InterpolationModeNone
		if interpolate {
			mode = policiesv1beta1.InterpolationModeCEL
		}
		compiled, errs := Compile(
			field.NewPath("spec", "generate").Index(0).Child("template"),
			env,
			&policiesv1beta1.GenerationTemplate{
				Value:       value,
				Interpolate: mode,
			},
		)
		if len(errs) > 0 {
			return // invalid YAML/CEL is expected fuzz input
		}
		if compiled == nil {
			t.Fatal("compilation succeeded but returned a nil template")
		}

		activation := map[string]any{
			"variables": map[string]any{
				"name":    "demo",
				"labels":  map[string]any{"team": "platform", "env": "test"},
				"names":   []any{"one", "two"},
				"enabled": true,
				"port":    int64(8080),
			},
			"object": map[string]any{"metadata": map[string]any{"name": "source"}},
		}

		first, err := compiled.Render(context.Background(), activation)
		if err != nil {
			return // dynamic values can legitimately fail during evaluation
		}
		second, err := compiled.Render(context.Background(), activation)
		if err != nil {
			t.Fatalf("previously successful render failed: %v", err)
		}

		// CEL may produce NaN, which is not reflexively equal. Do not
		// interpret IEEE-754 NaN semantics as nondeterminism.
		if containsNaN(first) || containsNaN(second) {
			return
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatal("identical activations produced different resource trees")
		}

		// Snapshot into test-owned storage: first and second might alias.
		// A shallow copy or the production deep-copy helper could hide a bug.
		baseline := snapshotIsolationValue(second).([]map[string]any)

		// Mutate the first result at the root and inside a nested map/list
		// when present. Subsequent renders must still equal the baseline.
		if len(first) > 0 {
			first[0]["__kyverno_fuzz_isolation_probe__"] = struct{}{}
			for _, resource := range first {
				if mutateNestedOutput(resource, 0) {
					break
				}
			}
		}
		if !reflect.DeepEqual(baseline, second) {
			t.Fatal("mutating the first render changed the second render")
		}
		third, err := compiled.Render(context.Background(), activation)
		if err != nil {
			t.Fatalf("render failed after output mutation: %v", err)
		}
		if !reflect.DeepEqual(baseline, third) {
			t.Fatal("rendered resources share mutable state across calls")
		}
	})
}

// snapshotIsolationValue is deliberately test-owned and does not call the
// production deepCopyValue helper whose behavior this fuzz target checks.
func snapshotIsolationValue(v any) any {
	switch v := v.(type) {
	case []map[string]any:
		if v == nil {
			return v
		}
		out := make([]map[string]any, len(v))
		for i, resource := range v {
			out[i] = snapshotIsolationValue(resource).(map[string]any)
		}
		return out
	case map[string]any:
		if v == nil {
			return v
		}
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[key] = snapshotIsolationValue(value)
		}
		return out
	case []any:
		if v == nil {
			return v
		}
		out := make([]any, len(v))
		for i, value := range v {
			out[i] = snapshotIsolationValue(value)
		}
		return out
	default:
		return v
	}
}

func containsNaN(v any) bool {
	switch v := v.(type) {
	case float64:
		return math.IsNaN(v)
	case []map[string]any:
		for _, resource := range v {
			if containsNaN(resource) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if containsNaN(item) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if containsNaN(item) {
				return true
			}
		}
	}
	return false
}

// This deterministic regression check ensures the fuzz oracles detect nested
// mutation and IEEE-754 NaN before running the fuzz corpus.
func TestGenerationTemplateFuzzOracles(t *testing.T) {
	t.Parallel()

	output := []map[string]any{
		{
			"metadata": map[string]any{
				"labels": map[string]any{"app": "original"},
			},
			"data": []any{map[string]any{"key": "value"}},
		},
	}
	snapshot := snapshotIsolationValue(output).([]map[string]any)
	output[0]["metadata"].(map[string]any)["labels"].(map[string]any)["app"] = "modified"
	output[0]["data"].([]any)[0].(map[string]any)["key"] = "changed"
	if !reflect.DeepEqual(snapshot[0]["metadata"], map[string]any{"labels": map[string]any{"app": "original"}}) {
		t.Fatal("snapshot shared a nested mapping")
	}
	if !reflect.DeepEqual(snapshot[0]["data"], []any{map[string]any{"key": "value"}}) {
		t.Fatal("snapshot shared a nested sequence")
	}
	if containsNaN(snapshot) {
		t.Fatal("finite resource contains NaN")
	}
	if !containsNaN([]map[string]any{{"data": []any{map[string]any{"double": math.NaN()}}}}) {
		t.Fatal("failed to detect NaN nested within a resource slice")
	}
}

// mutateNestedOutput modifies an existing nested mapping or sequence.
// The depth limit is a harness guard, not a limit on production templates.
func mutateNestedOutput(v any, depth int) bool {
	if depth > 64 {
		return false
	}
	switch v := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if mutateNestedOutput(v[key], depth+1) {
				return true
			}
		}
		if depth > 0 {
			v["__kyverno_fuzz_nested_probe__"] = struct{}{}
			return true
		}
	case []any:
		for _, item := range v {
			if mutateNestedOutput(item, depth+1) {
				return true
			}
		}
		if len(v) > 0 {
			v[0] = struct{}{}
			return true
		}
	}
	return false
}
