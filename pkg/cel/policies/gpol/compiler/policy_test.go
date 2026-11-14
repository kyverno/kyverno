package compiler

import (
	"context"
	"fmt"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	v1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
)

type mockProgram struct {
	retVal ref.Val
	err    error
}

func (m *mockProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return m.retVal, nil, m.err
}
func (m *mockProgram) Eval(any) (ref.Val, *cel.EvalDetails, error) {
	return m.retVal, nil, m.err
}
func (m *mockProgram) ConcurrentEval(_ context.Context, _ any) <-chan cel.EvalResult { return nil }

var (
	gvk     = schema.GroupVersionKind{Group: "", Version: "", Kind: ""}
	request = engine.Request(&libs.FakeContextProvider{}, res.GroupVersionKind(), schema.GroupVersionResource{}, "", "", "", admissionv1.Create, authenticationv1.UserInfo{}, &res, nil, false, nil)
	attr    = admission.NewAttributesRecord(&obj, &oldObj, gvk, "", "", gvk.GroupVersion().WithResource("res"), "", admission.Connect, &res, false, &user.DefaultInfo{})
)

func TestPolicyEvaluate(t *testing.T) {
	t.Run("returns empty result when generations and conditions are valid", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations:     []Generation{},
			exceptions:      []compiler.Exception{},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("valid-name")
		res.SetNamespace("test-ns")

		result, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.NotNil(t, result)
		assert.Nil(t, result.GeneratedResources)
		assert.Nil(t, result.Exceptions)
		assert.NoError(t, err)
	})

	t.Run("returns exception if policyException matches", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations: []Generation{
				{expression: &mockProgram{retVal: types.String("value")}},
			},
			exceptions: []compiler.Exception{
				{
					MatchConditions: []cel.Program{},
					Exception: &v1beta1.PolicyException{
						Spec: v1beta1.PolicyExceptionSpec{
							MatchConditions: []v1.MatchCondition{
								{Name: "valid", Expression: "object.metadata.namespace == 'test-ns'"},
							},
						},
					},
				},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("exception-name")
		res.SetNamespace("test-ns")

		result, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.NotNil(t, result)
		assert.NotNil(t, result.Exceptions)
		assert.NoError(t, err)
	})

	t.Run("returns error if exception matchCondition fails evaluation", func(t *testing.T) {
		policy := &Policy{
			exceptions: []compiler.Exception{
				{
					MatchConditions: []cel.Program{
						&mockProgram{retVal: types.String("not-a-bool")}, // triggers convertToNative error
					},
				},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("bad-exception")
		res.SetNamespace("bad-ns")

		result, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.Nil(t, result)
		assert.Error(t, err)
	})

	t.Run("returns error if main matchCondition fails evaluation", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{
				&mockProgram{retVal: types.String("bad-type")},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("bad-match")
		res.SetNamespace("ns")

		result, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.Nil(t, result)
		assert.Error(t, err)
	})

	t.Run("returns error if generation expression fails evaluation", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations: []Generation{
				{expression: &mockProgram{err: fmt.Errorf("generation error")}},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("gen-fail")
		res.SetNamespace("ns")

		_, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})
		assert.Error(t, err)
	})

	t.Run("returns audit annotations in evaluation result", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations:     []Generation{},
			auditAnnotations: map[string]cel.Program{
				"env": &mockProgram{retVal: types.String("production")},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("audit-name")
		res.SetNamespace("test-ns")

		result, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "production", result.AuditAnnotations["env"])
	})

	t.Run("returns error when audit annotation expression fails", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations:     []Generation{},
			auditAnnotations: map[string]cel.Program{
				"broken": &mockProgram{err: fmt.Errorf("annotation eval error")},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("audit-err")
		res.SetNamespace("ns")

		_, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})
		assert.Error(t, err)
	})

	t.Run("full-exemption exception takes precedence over partial exceptions", func(t *testing.T) {
		// Regression test for https://github.com/kyverno/kyverno/issues/16053:
		// When both a partial exception and a full-exemption exception match, the
		// full exemption must be honoured and evaluation must be skipped entirely.
		// The loop breaks on the full exemption and resets any previously accumulated
		// partial scopes (allowedImages / allowedValues).
		partialException := &v1beta1.PolicyException{
			Spec: v1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}
		fullExemptionException := &v1beta1.PolicyException{
			Spec: v1beta1.PolicyExceptionSpec{
				// empty Images and AllowedValues → full exemption
			},
		}
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations:     []Generation{},
			exceptions: []compiler.Exception{
				// partial exception is evaluated first
				{MatchConditions: []cel.Program{}, Exception: partialException},
				// full exemption is evaluated second; the loop breaks here and resets partial scopes
				{MatchConditions: []cel.Program{}, Exception: fullExemptionException},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("mixed-exceptions")
		res.SetNamespace("test-ns")

		result, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.NoError(t, err)
		// The full exemption breaks the loop (resetting partial scopes); the
		// post-loop check returns with the collected exceptions.
		assert.NotNil(t, result)
		assert.Nil(t, result.GeneratedResources)
		assert.NotEmpty(t, result.Exceptions)
	})

	t.Run("full-exemption first, partial second — full exemption wins", func(t *testing.T) {
		// Verifies the break+reset approach works regardless of ordering.
		fullExemptionException := &v1beta1.PolicyException{
			Spec: v1beta1.PolicyExceptionSpec{
				// empty Images and AllowedValues → full exemption
			},
		}
		partialException := &v1beta1.PolicyException{
			Spec: v1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations:     []Generation{},
			exceptions: []compiler.Exception{
				// full exemption first: loop breaks immediately, partial never reached
				{MatchConditions: []cel.Program{}, Exception: fullExemptionException},
				{MatchConditions: []cel.Program{}, Exception: partialException},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("full-first-exceptions")
		res.SetNamespace("test-ns")

		result, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Nil(t, result.GeneratedResources)
		assert.NotEmpty(t, result.Exceptions)
	})
}

type recordingGenerateContext struct {
	*libs.FakeContextProvider
	generateCtx context.Context
}

func (c *recordingGenerateContext) GenerateResourcesContext(ctx context.Context, namespace string, dataList []map[string]any) error {
	c.generateCtx = ctx
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.FakeContextProvider.GenerateResources(namespace, dataList)
}

func TestPolicyEvaluate_GeneratorBindingUsesRequestContext(t *testing.T) {
	pol := &v1beta1.GeneratingPolicy{
		Spec: v1beta1.GeneratingPolicySpec{
			Generation: []v1beta1.Generation{{
				Expression: `generator.Apply("test-ns", [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": "gen-cm"})}])`,
			}},
		},
	}
	compiled, errs := NewCompiler().Compile(pol, nil)
	require.Empty(t, errs)

	type testCtxKey struct{}

	t.Run("propagates request context to generator expression", func(t *testing.T) {
		provider := &recordingGenerateContext{FakeContextProvider: libs.NewFakeContextProvider()}
		ctx := context.WithValue(context.Background(), testCtxKey{}, "request-val")

		result, err := compiled.Evaluate(ctx, attr, &request.Request, &ns, provider)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.GeneratedResources, 1)
		require.NotNil(t, provider.generateCtx)
		assert.Equal(t, "request-val", provider.generateCtx.Value(testCtxKey{}))
	})

	t.Run("cancelled request context fails generator evaluation", func(t *testing.T) {
		provider := &recordingGenerateContext{FakeContextProvider: libs.NewFakeContextProvider()}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result, err := compiled.Evaluate(ctx, attr, &request.Request, &ns, provider)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("template generation propagates request context and honors cancellation", func(t *testing.T) {
		tmplPol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				Generation: []v1beta1.Generation{{
					Template: &v1beta1.GenerationTemplate{
						Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: tmpl-cm
  namespace: test-ns
`,
					},
				}},
			},
		}
		compiledTmpl, errs := NewCompiler().Compile(tmplPol, nil)
		require.Empty(t, errs)

		provider := &recordingGenerateContext{FakeContextProvider: libs.NewFakeContextProvider()}
		ctx := context.WithValue(context.Background(), testCtxKey{}, "template-val")

		result, err := compiledTmpl.Evaluate(ctx, attr, &request.Request, &ns, provider)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.GeneratedResources, 1)
		require.NotNil(t, provider.generateCtx)
		assert.Equal(t, "template-val", provider.generateCtx.Value(testCtxKey{}))

		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err = compiledTmpl.Evaluate(cancelledCtx, attr, &request.Request, &ns, provider)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
