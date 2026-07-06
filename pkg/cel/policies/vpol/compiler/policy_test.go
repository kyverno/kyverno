package compiler

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	testGVK     = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	testObj     = unstructured.Unstructured{}
	testOldObj  = unstructured.Unstructured{}
	testNs      = unstructured.Unstructured{}
	testRes     = unstructured.Unstructured{}
	testRequest = engine.Request(
		&libs.FakeContextProvider{},
		testRes.GroupVersionKind(),
		schema.GroupVersionResource{},
		"",
		"",
		"",
		admissionv1.Create,
		authenticationv1.UserInfo{},
		&testRes,
		nil,
		false,
		nil,
	)
	testAttr = admission.NewAttributesRecord(
		&testObj,
		&testOldObj,
		testGVK,
		"",
		"",
		testGVK.GroupVersion().WithResource("pods"),
		"",
		admission.Create,
		&testRes,
		false,
		&user.DefaultInfo{},
	)
)

func TestPolicyEvaluate(t *testing.T) {
	t.Run("all validations pass in json mode", func(t *testing.T) {
		policy := &Policy{
			mode: policieskyvernoio.EvaluationModeJSON,
			validations: []compiler.Validation{
				{Program: &mockProgram{retVal: types.Bool(true)}},
			},
		}

		result, err := policy.Evaluate(context.TODO(), map[string]any{"name": "allowed"}, nil, nil, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Result)
	})

	t.Run("validation fails in json mode", func(t *testing.T) {
		policy := &Policy{
			mode: policieskyvernoio.EvaluationModeJSON,
			validations: []compiler.Validation{
				{
					Message: "denied",
					Program: &mockProgram{retVal: types.Bool(false)},
				},
			},
		}

		result, err := policy.Evaluate(context.TODO(), map[string]any{"name": "denied"}, nil, nil, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Result)
		assert.Equal(t, "denied", result.Message)
		assert.Equal(t, 0, result.Index)
	})

	t.Run("validation program error in json mode", func(t *testing.T) {
		policy := &Policy{
			mode: policieskyvernoio.EvaluationModeJSON,
			validations: []compiler.Validation{
				{Program: &mockProgram{err: fmt.Errorf("eval error")}},
			},
		}

		result, err := policy.Evaluate(context.TODO(), map[string]any{"name": "test"}, nil, nil, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Error(t, result.Error)
		assert.Equal(t, 0, result.Index)
	})

	t.Run("match condition not met", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{
				&mockProgram{retVal: types.Bool(false)},
			},
			validations: []compiler.Validation{
				{Program: &mockProgram{retVal: types.Bool(true)}},
			},
		}

		result, err := policy.Evaluate(context.TODO(), nil, testAttr, &testRequest.Request, &testNs, &libs.FakeContextProvider{})
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("match condition eval error with fail policy", func(t *testing.T) {
		policy := &Policy{
			failurePolicy: admissionregistrationv1.Fail,
			matchConditions: []cel.Program{
				&mockProgram{retVal: types.String("not-a-bool")},
			},
		}

		result, err := policy.Evaluate(context.TODO(), nil, testAttr, &testRequest.Request, &testNs, &libs.FakeContextProvider{})
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("match condition eval error with ignore policy", func(t *testing.T) {
		policy := &Policy{
			failurePolicy: admissionregistrationv1.Ignore,
			matchConditions: []cel.Program{
				&mockProgram{retVal: types.String("not-a-bool")},
			},
		}

		result, err := policy.Evaluate(context.TODO(), nil, testAttr, &testRequest.Request, &testNs, &libs.FakeContextProvider{})
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("exception matches and skips evaluation", func(t *testing.T) {
		policy := &Policy{
			matchConditions: []cel.Program{},
			validations: []compiler.Validation{
				{Program: &mockProgram{retVal: types.Bool(false)}},
			},
			exceptions: []compiler.Exception{
				{
					MatchConditions: []cel.Program{
						&mockProgram{retVal: types.Bool(true)},
					},
					Exception: &policiesv1beta1.PolicyException{
						ObjectMeta: metav1.ObjectMeta{Name: "exc1"},
					},
				},
			},
		}

		result, err := policy.Evaluate(context.TODO(), nil, testAttr, &testRequest.Request, &testNs, &libs.FakeContextProvider{})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Exceptions, 1)
	})

	t.Run("exception matchCondition eval error", func(t *testing.T) {
		policy := &Policy{
			exceptions: []compiler.Exception{
				{
					MatchConditions: []cel.Program{
						&mockProgram{retVal: types.String("not-a-bool")},
					},
				},
			},
		}

		result, err := policy.Evaluate(context.TODO(), nil, testAttr, &testRequest.Request, &testNs, &libs.FakeContextProvider{})
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("audit annotations on success", func(t *testing.T) {
		policy := &Policy{
			mode:            policieskyvernoio.EvaluationModeJSON,
			matchConditions: []cel.Program{},
			validations: []compiler.Validation{
				{Program: &mockProgram{retVal: types.Bool(true)}},
			},
			auditAnnotations: map[string]cel.Program{
				"owner": &mockProgram{retVal: types.String("team-a")},
			},
		}

		result, err := policy.Evaluate(context.TODO(), map[string]any{"name": "allowed"}, nil, nil, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Result)
		assert.Equal(t, map[string]string{"owner": "team-a"}, result.AuditAnnotations)
	})
}
