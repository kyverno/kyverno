package compiler

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	v1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
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
