package template

import (
	"context"
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func testEnv(t *testing.T) *cel.Env {
	t.Helper()
	env, err := cel.NewEnv(
		cel.Variable("variables", cel.DynType),
		cel.Variable("object", cel.DynType),
	)
	assert.NoError(t, err)
	return env
}

func compileTemplate(t *testing.T, value string, interpolate policiesv1beta1.InterpolationMode) (*Template, field.ErrorList) {
	t.Helper()
	return Compile(field.NewPath("spec", "generate").Index(0).Child("template"), testEnv(t), &policiesv1beta1.GenerationTemplate{
		Value:       value,
		Interpolate: interpolate,
	})
}

func TestCompileAndRender(t *testing.T) {
	activation := map[string]any{
		"variables": map[string]any{
			"targetName":      "myns",
			"targetNamespace": "myns",
			"port":            int64(8080),
			"enabled":         true,
			"selector": map[string]any{
				"matchLabels": map[string]any{"app": "web"},
			},
			"labels": map[string]any{"team": "dev", "env": "prod"},
			"nsList": []any{"a", "b"},
		},
	}
	tests := []struct {
		name        string
		value       string
		interpolate policiesv1beta1.InterpolationMode
		want        []map[string]any
		wantErr     string
	}{{
		name: "plain yaml no interpolation",
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: static
  namespace: default
data:
  key: value
`,
		want: []map[string]any{{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "static", "namespace": "default"},
			"data":       map[string]any{"key": "value"},
		}},
	}, {
		name:        "placeholders inert without interpolation",
		interpolate: policiesv1beta1.InterpolationModeNone,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.targetName ))
`,
		want: []map[string]any{{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "(( variables.targetName ))"},
		}},
	}, {
		name:        "scalar interpolation",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.targetName ))
  namespace: (( variables.targetNamespace ))
`,
		want: []map[string]any{{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "myns", "namespace": "myns"},
		}},
	}, {
		name:        "string interpolation with prefix and suffix",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.targetName ))-scraping
`,
		want: []map[string]any{{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "myns-scraping"},
		}},
	}, {
		name:        "structural splice of map",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: cnp
spec:
  endpointSelector: (( variables.selector ))
`,
		want: []map[string]any{{
			"apiVersion": "cilium.io/v2",
			"kind":       "CiliumNetworkPolicy",
			"metadata":   map[string]any{"name": "cnp"},
			"spec": map[string]any{
				"endpointSelector": map[string]any{
					"matchLabels": map[string]any{"app": "web"},
				},
			},
		}},
	}, {
		name:        "structural splice of map and list",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  labels: (( variables.labels ))
  finalizers: (( variables.nsList ))
`,
		want: []map[string]any{{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":       "cm",
				"labels":     map[string]any{"team": "dev", "env": "prod"},
				"finalizers": []any{"a", "b"},
			},
		}},
	}, {
		name:        "mixed static and interpolated array elements",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: cnp
spec:
  ingress:
  - toPorts:
    - ports:
      - port: (( string(variables.port) ))
        protocol: TCP
`,
		want: []map[string]any{{
			"apiVersion": "cilium.io/v2",
			"kind":       "CiliumNetworkPolicy",
			"metadata":   map[string]any{"name": "cnp"},
			"spec": map[string]any{
				"ingress": []any{
					map[string]any{
						"toPorts": []any{
							map[string]any{
								"ports": []any{
									map[string]any{"port": "8080", "protocol": "TCP"},
								},
							},
						},
					},
				},
			},
		}},
	}, {
		name:        "non-string scalar splice keeps type",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: Service
metadata:
  name: svc
spec:
  port: (( variables.port ))
  enabled: (( variables.enabled ))
`,
		want: []map[string]any{{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]any{"name": "svc"},
			"spec":       map[string]any{"port": int64(8080), "enabled": true},
		}},
	}, {
		name:        "multi document",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: first
  namespace: (( variables.targetNamespace ))
---
apiVersion: v1
kind: Secret
metadata:
  name: second
  namespace: other
`,
		want: []map[string]any{{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "first", "namespace": "myns"},
		}, {
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata":   map[string]any{"name": "second", "namespace": "other"},
		}},
	}, {
		name:        "escaped placeholder stays literal",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
data:
  key: \(( literal ))
`,
		want: []map[string]any{{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "cm"},
			"data":       map[string]any{"key": "(( literal ))"},
		}},
	}, {
		name:        "embedded placeholder rejects structured result",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: prefix-(( variables.labels ))
`,
		wantErr: "must evaluate to a scalar",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl, errs := compileTemplate(t, tt.value, tt.interpolate)
			require.Empty(t, errs)
			got, err := tpl.Render(context.TODO(), activation)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			want := make([]any, 0, len(tt.want))
			for _, m := range tt.want {
				want = append(want, any(m))
			}
			gotAny := make([]any, 0, len(got))
			for _, m := range got {
				gotAny = append(gotAny, any(m))
			}
			assert.Equal(t, want, gotAny)
		})
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		interpolate policiesv1beta1.InterpolationMode
		wantErr     string
	}{{
		name:    "malformed yaml",
		value:   "apiVersion: v1\nkind: [unclosed",
		wantErr: "failed to parse YAML",
	}, {
		name:    "empty template",
		value:   "",
		wantErr: "at least one YAML document",
	}, {
		name:    "non-mapping document",
		value:   "- a\n- b",
		wantErr: "must be a mapping",
	}, {
		name:        "invalid CEL expression",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables. ))
`,
		wantErr: "invalid placeholder expression",
	}, {
		name:        "unterminated placeholder",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: "(( variables.name"
`,
		wantErr: "unterminated placeholder",
	}, {
		name:        "placeholder in mapping key",
		interpolate: policiesv1beta1.InterpolationModeCEL,
		value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  labels:
    (( variables.key )): enabled
`,
		wantErr: "placeholders are not supported in mapping keys",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := compileTemplate(t, tt.value, tt.interpolate)
			assert.NotEmpty(t, errs)
			assert.Contains(t, errs.ToAggregate().Error(), tt.wantErr)
		})
	}
}

func TestRenderIsolation(t *testing.T) {
	// rendered resources must not share state across renders
	tpl, errs := compileTemplate(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: static
data:
  nested:
    key: value
`, policiesv1beta1.InterpolationModeNone)
	require.Empty(t, errs)
	first, err := tpl.Render(context.TODO(), map[string]any{})
	assert.NoError(t, err)
	first[0]["mutated"] = true
	nested := first[0]["data"].(map[string]any)["nested"].(map[string]any)
	nested["key"] = "mutated"
	second, err := tpl.Render(context.TODO(), map[string]any{})
	assert.NoError(t, err)
	_, mutated := second[0]["mutated"]
	assert.False(t, mutated)
	assert.Equal(t, "value", second[0]["data"].(map[string]any)["nested"].(map[string]any)["key"])
}

func TestExtractExpressions(t *testing.T) {
	value := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.a ))
  namespace: prefix-(( variables.b ))-(( variables.c ))
data:
  static: value
`
	// malformed YAML yields no expressions, even if earlier documents parsed
	assert.Empty(t, ExtractExpressions("metadata:\n  name: (( variables.a ))\n---\nkind: [unclosed"))
	assert.Equal(t, []string{"variables.a", "variables.b", "variables.c"}, ExtractExpressions(value))
	assert.Empty(t, ExtractExpressions("plain: yaml"))
}
