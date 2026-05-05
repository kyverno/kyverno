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
	t.Run("returns nil when generations and conditions are valid", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations:     []cel.Program{},
			exceptions:      []compiler.Exception{},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("valid-name")
		res.SetNamespace("test-ns")

		resources, exceptions, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.Nil(t, resources)
		assert.Nil(t, exceptions)
		assert.NoError(t, err)
	})

	t.Run("returns exception if policyException matches", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations: []cel.Program{
				&mockProgram{retVal: types.String("value")},
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

		resources, exceptions, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.Nil(t, resources)
		assert.NotNil(t, exceptions)
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

		resources, exceptions, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.Nil(t, resources)
		assert.Nil(t, exceptions)
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

		resources, exceptions, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})

		assert.Nil(t, resources)
		assert.Nil(t, exceptions)
		assert.Error(t, err)
	})

	t.Run("returns error if generation expression fails evaluation", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			variables:       map[string]cel.Program{},
			generations: []cel.Program{
				&mockProgram{err: fmt.Errorf("generation error")},
			},
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("gen-fail")
		res.SetNamespace("ns")

		_, _, err := policy.Evaluate(context.TODO(), attr, &request.Request, &ns, &libs.FakeContextProvider{})
		assert.Error(t, err)
	})
}
