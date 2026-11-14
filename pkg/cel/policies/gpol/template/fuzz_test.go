package template

import (
	"context"
	"fmt"
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
		{`apiVersion: v1
kind: ConfigMap
metadata:
  name: nan-and-nested
  annotations:
    numeric: .nan
  labels:
    app: example
data:
  nested:
    - key: value
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

		// YAML may contain NaN. Compare its scalar values by IEEE-754
		// NaN semantics without skipping isolation checks for other fields.
		if !equalFuzzRenderValues(first, second) {
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
		if !equalFuzzRenderValues(baseline, second) {
			t.Fatal("mutating the first render changed the second render")
		}
		third, err := compiled.Render(context.Background(), activation)
		if err != nil {
			t.Fatalf("render failed after output mutation: %v", err)
		}
		if !equalFuzzRenderValues(baseline, third) {
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
	case map[any]any:
		// Non-string YAML mapping keys can occur in literal subtrees.
		if v == nil {
			return v
		}
		out := make(map[any]any, len(v))
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

// equalFuzzRenderValues compares YAML resource trees, treating two NaNs as
// equal while continuing to compare their surrounding maps and sequences.
// Unlike reflect.DeepEqual on aliased maps, the independent snapshot makes
// mutations visible. Unknown leaf types retain normal DeepEqual semantics.
func equalFuzzRenderValues(a, b any) bool {
	switch a := a.(type) {
	case float64:
		b, ok := b.(float64)
		return ok && (a == b || (math.IsNaN(a) && math.IsNaN(b)))
	case []map[string]any:
		b, ok := b.([]map[string]any)
		if !ok || (a == nil) != (b == nil) || len(a) != len(b) {
			return false
		}
		for i := range a {
			if !equalFuzzRenderValues(a[i], b[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		b, ok := b.(map[string]any)
		if !ok || (a == nil) != (b == nil) || len(a) != len(b) {
			return false
		}
		for key, value := range a {
			other, exists := b[key]
			if !exists || !equalFuzzRenderValues(value, other) {
				return false
			}
		}

		return true
	case map[any]any:
		b, ok := b.(map[any]any)
		if !ok || (a == nil) != (b == nil) || len(a) != len(b) {
			return false
		}
		// Compare key/value pairs, rather than indexing by key: a YAML NaN
		// mapping key is non-reflexive and cannot be looked up in a Go map.
		type entry struct{ key, value any }
		remaining := make([]entry, 0, len(b))
		for key, value := range b {
			remaining = append(remaining, entry{key, value})
		}
		for key, value := range a {
			matched := -1
			for i, candidate := range remaining {
				if equalFuzzRenderValues(key, candidate.key) && equalFuzzRenderValues(value, candidate.value) {
					matched = i
					break
				}
			}
			if matched < 0 {
				return false
			}
			remaining = append(remaining[:matched], remaining[matched+1:]...)
		}
		return true
	case []any:
		b, ok := b.([]any)
		if !ok || (a == nil) != (b == nil) || len(a) != len(b) {
			return false
		}
		for i := range a {
			if !equalFuzzRenderValues(a[i], b[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
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
	// NaN must be reflexively comparable without excluding nearby mutable
	// maps or lists from the oracle.
	nanResources := []map[string]any{{
		"data": []any{map[string]any{"double": math.NaN(), "name": "original"}},
	}}
	nanSnapshot := snapshotIsolationValue(nanResources).([]map[string]any)
	if !equalFuzzRenderValues(nanResources, nanSnapshot) {
		t.Fatal("equivalent resource trees containing NaN must compare equal")
	}
	nanResources[0]["data"].([]any)[0].(map[string]any)["name"] = "changed"
	if equalFuzzRenderValues(nanResources, nanSnapshot) {
		t.Fatal("NaN comparison hid a nested mutation")
	}
	if equalFuzzRenderValues(
		[]map[string]any{{"value": math.NaN()}},
		[]map[string]any{{"value": 1.0}},
	) {
		t.Fatal("NaN must not compare equal to a finite float")
	}

	// Two distinct roots deliberately share only a nested map. The nested
	// probe must mutate that shared state, and the independent snapshot must
	// detect the change even though the root maps remain separate.
	shared := map[string]any{"list": []any{map[string]any{"name": "original"}}}
	first := []map[string]any{{"nested": shared}}
	second := []map[string]any{{"nested": shared}}
	baseline := snapshotIsolationValue(second).([]map[string]any)
	if !mutateNestedOutput(first[0], 0) {
		t.Fatal("nested isolation probe did not mutate output")
	}
	if equalFuzzRenderValues(baseline, second) {
		t.Fatal("nested probe failed to expose shared state")
	}
	if !equalFuzzRenderValues(baseline, snapshotIsolationValue(baseline)) {
		t.Fatal("snapshot changed while testing shared state")
	}

	// A literal YAML subtree may decode into map[any]any when it has
	// non-string keys. A shared map behind that shape must not evade the
	// independent snapshot, NaN-aware equality, or nested mutation probe.
	sharedMixed := map[any]any{
		int64(7): []any{map[any]any{
			true:      map[string]any{"name": "original"},
			"numeric": math.NaN(),
		}},
	}
	firstMixed := []map[string]any{{"nested": sharedMixed}}
	secondMixed := []map[string]any{{"nested": sharedMixed}}
	mixedBaseline := snapshotIsolationValue(secondMixed).([]map[string]any)
	if !equalFuzzRenderValues(mixedBaseline, secondMixed) {
		t.Fatal("independent snapshot of a non-string-key map differs")
	}
	if !mutateNestedOutput(firstMixed[0], 0) {
		t.Fatal("nested probe failed to traverse a non-string-key map")
	}
	if equalFuzzRenderValues(mixedBaseline, secondMixed) {
		t.Fatal("nested non-string-key map sharing was not detected")
	}
	if !equalFuzzRenderValues(mixedBaseline, snapshotIsolationValue(mixedBaseline)) {
		t.Fatal("snapshot of a non-string-key map changed")
	}
	if !equalFuzzRenderValues(
		map[any]any{math.NaN(): "value"},
		map[any]any{math.NaN(): "value"},
	) {
		t.Fatal("equal non-reflexive map keys must compare equal")
	}
	if equalFuzzRenderValues(
		map[any]any{math.NaN(): "value"},
		map[any]any{math.NaN(): "changed"},
	) {
		t.Fatal("comparison ignored a value behind a non-reflexive map key")
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
	case map[any]any:
		// Iterate the decoded values, not v[key]: NaN keys cannot be looked
		// up even when they were obtained from the same map.
		type entry struct {
			sortKey string
			value   any
		}
		entries := make([]entry, 0, len(v))
		for key, value := range v {
			entries = append(entries, entry{fmt.Sprintf("%T:%#v", key, key), value})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].sortKey < entries[j].sortKey
		})
		for _, item := range entries {
			if mutateNestedOutput(item.value, depth+1) {
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
