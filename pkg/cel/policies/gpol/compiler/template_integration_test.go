package compiler

import (
	"context"
	"testing"

	v1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompileTemplateGeneration(t *testing.T) {
	newPolicy := func(generation v1beta1.Generation) *v1beta1.GeneratingPolicy {
		return &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				Variables: []admissionregistrationv1.Variable{
					{Name: "nsName", Expression: "object.metadata.name"},
				},
				Generation: []v1beta1.Generation{generation},
			},
		}
	}
	t.Run("compiles template with cel interpolation", func(t *testing.T) {
		pol := newPolicy(v1beta1.Generation{
			Template: &v1beta1.GenerationTemplate{
				Interpolate: v1beta1.InterpolationModeCEL,
				Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.nsName ))
  namespace: (( variables.nsName ))
`,
			},
		})
		compiled, errs := NewCompiler().Compile(pol, nil)
		assert.Nil(t, errs)
		assert.NotNil(t, compiled)
	})
	t.Run("rejects entry with both expression and template", func(t *testing.T) {
		pol := newPolicy(v1beta1.Generation{
			Expression: `generator.Apply("ns", [])`,
			Template:   &v1beta1.GenerationTemplate{Value: "apiVersion: v1\nkind: ConfigMap"},
		})
		compiled, errs := NewCompiler().Compile(pol, nil)
		assert.Nil(t, compiled)
		assert.Contains(t, errs.ToAggregate().Error(), "only one of expression or template")
	})
	t.Run("rejects entry with neither expression nor template", func(t *testing.T) {
		pol := newPolicy(v1beta1.Generation{})
		compiled, errs := NewCompiler().Compile(pol, nil)
		assert.Nil(t, compiled)
		assert.Contains(t, errs.ToAggregate().Error(), "one of expression or template must be set")
	})
	t.Run("rejects invalid placeholder expression with template path", func(t *testing.T) {
		pol := newPolicy(v1beta1.Generation{
			Template: &v1beta1.GenerationTemplate{
				Interpolate: v1beta1.InterpolationModeCEL,
				Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.unknown.bad. ))
`,
			},
		})
		compiled, errs := NewCompiler().Compile(pol, nil)
		assert.Nil(t, compiled)
		assert.Contains(t, errs.ToAggregate().Error(), "spec.generate[0].template.value")
		assert.Contains(t, errs.ToAggregate().Error(), "invalid placeholder expression")
	})
	t.Run("rejects placeholder in mapping key", func(t *testing.T) {
		pol := newPolicy(v1beta1.Generation{
			Template: &v1beta1.GenerationTemplate{
				Interpolate: v1beta1.InterpolationModeCEL,
				Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
data:
  (( variables.nsName )): value
`,
			},
		})
		compiled, errs := NewCompiler().Compile(pol, nil)
		assert.Nil(t, compiled)
		assert.Contains(t, errs.ToAggregate().Error(), "placeholders are not supported in mapping keys")
	})
}

func TestEvaluateTemplateGeneration(t *testing.T) {
	obj.SetName("trigger")
	obj.SetNamespace("default")
	res.SetName("trigger")
	res.SetNamespace("default")

	evaluate := func(t *testing.T, policy v1beta1.GeneratingPolicyLike) (*EvaluationResult, error) {
		t.Helper()
		compiled, errs := NewCompiler().Compile(policy, nil)
		require.Nil(t, errs)
		return compiled.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})
	}

	t.Run("template mode generates resources through the generation runtime", func(t *testing.T) {
		pol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				Variables: []admissionregistrationv1.Variable{
					{Name: "targetName", Expression: "object.metadata.name"},
				},
				Generation: []v1beta1.Generation{{
					Template: &v1beta1.GenerationTemplate{
						Interpolate: v1beta1.InterpolationModeCEL,
						Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( variables.targetName ))-config
  namespace: target-ns
data:
  key: value
`,
					},
				}},
			},
		}
		result, err := evaluate(t, pol)
		require.NoError(t, err)
		require.Len(t, result.GeneratedResources, 1)
		generated := result.GeneratedResources[0]
		assert.Equal(t, "ConfigMap", generated.GetKind())
		assert.Equal(t, "trigger-config", generated.GetName())
		assert.Equal(t, "target-ns", generated.GetNamespace())
	})

	t.Run("template mode matches expression mode output", func(t *testing.T) {
		templatePol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				Generation: []v1beta1.Generation{{
					Template: &v1beta1.GenerationTemplate{
						Interpolate: v1beta1.InterpolationModeCEL,
						Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( object.metadata.name ))
  namespace: target-ns
`,
					},
				}},
			},
		}
		expressionPol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				Generation: []v1beta1.Generation{{
					Expression: `generator.Apply("target-ns", [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": object.metadata.name})}])`,
				}},
			},
		}
		templateResult, err := evaluate(t, templatePol)
		require.NoError(t, err)
		expressionResult, err := evaluate(t, expressionPol)
		require.NoError(t, err)
		require.Len(t, templateResult.GeneratedResources, 1)
		require.Len(t, expressionResult.GeneratedResources, 1)
		assert.Equal(t, expressionResult.GeneratedResources[0].GetKind(), templateResult.GeneratedResources[0].GetKind())
		assert.Equal(t, expressionResult.GeneratedResources[0].GetName(), templateResult.GeneratedResources[0].GetName())
		assert.Equal(t, expressionResult.GeneratedResources[0].GetNamespace(), templateResult.GeneratedResources[0].GetNamespace())
	})

	t.Run("multi-document template targets each document namespace", func(t *testing.T) {
		pol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				Generation: []v1beta1.Generation{{
					Template: &v1beta1.GenerationTemplate{
						Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: first
  namespace: ns-a
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: second
  namespace: ns-b
`,
					},
				}},
			},
		}
		result, err := evaluate(t, pol)
		require.NoError(t, err)
		require.Len(t, result.GeneratedResources, 2)
		assert.Equal(t, "ns-a", result.GeneratedResources[0].GetNamespace())
		assert.Equal(t, "ns-b", result.GeneratedResources[1].GetNamespace())
	})

	t.Run("interpolate none keeps placeholders literal", func(t *testing.T) {
		pol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				Generation: []v1beta1.Generation{{
					Template: &v1beta1.GenerationTemplate{
						Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: static
  namespace: target-ns
data:
  key: (( not.evaluated ))
`,
					},
				}},
			},
		}
		result, err := evaluate(t, pol)
		require.NoError(t, err)
		require.Len(t, result.GeneratedResources, 1)
		value, _, err := unstructuredNestedString(result.GeneratedResources[0].Object, "data", "key")
		require.NoError(t, err)
		assert.Equal(t, "(( not.evaluated ))", value)
	})

	t.Run("namespaced policy denies cross-namespace template targets", func(t *testing.T) {
		pol := &v1beta1.NamespacedGeneratingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "tenant-pol", Namespace: "tenant-ns"},
			Spec: v1beta1.GeneratingPolicySpec{
				Generation: []v1beta1.Generation{{
					Template: &v1beta1.GenerationTemplate{
						Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: escape
  namespace: kube-system
`,
					},
				}},
			},
		}
		_, err := evaluate(t, pol)
		require.ErrorContains(t, err, "cross-namespace generation denied")
	})

	t.Run("namespaced policy defaults template target to policy namespace", func(t *testing.T) {
		pol := &v1beta1.NamespacedGeneratingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "tenant-pol", Namespace: "tenant-ns"},
			Spec: v1beta1.GeneratingPolicySpec{
				Generation: []v1beta1.Generation{{
					Template: &v1beta1.GenerationTemplate{
						Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: tenant-config
`,
					},
				}},
			},
		}
		result, err := evaluate(t, pol)
		require.NoError(t, err)
		require.Len(t, result.GeneratedResources, 1)
		assert.Equal(t, "tenant-ns", result.GeneratedResources[0].GetNamespace())
	})
}

func unstructuredNestedString(obj map[string]any, fields ...string) (string, bool, error) {
	current := any(obj)
	for _, field := range fields {
		m, ok := current.(map[string]any)
		if !ok {
			return "", false, nil
		}
		current, ok = m[field]
		if !ok {
			return "", false, nil
		}
	}
	s, ok := current.(string)
	return s, ok, nil
}
