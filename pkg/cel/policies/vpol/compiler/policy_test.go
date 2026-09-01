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
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

// mockVpolProgram is a lightweight cel.Program stub for unit tests.
type mockVpolProgram struct {
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
func (m *mockVpolProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return m.retVal, nil, m.err
}

func (m *mockVpolProgram) Eval(any) (ref.Val, *cel.EvalDetails, error) {
	return m.retVal, nil, m.err
}

func (m *mockVpolProgram) ConcurrentEval(_ context.Context, _ any) <-chan cel.EvalResult {
	return nil
}

// TestEvaluateWithData_FullExemptionPrecedence is a regression test for
// https://github.com/kyverno/kyverno/issues/16053.
//
// When multiple PolicyExceptions match a resource, a full-exemption exception
// (one with no Images and no AllowedValues) must cause the evaluation loop to
// break and indicate a full exemption, regardless of whether a partial
// exception also matched.
func TestEvaluateWithData_FullExemptionPrecedence(t *testing.T) {
	t.Run("full-exemption takes precedence over partial exception", func(t *testing.T) {
		partialEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}
		fullEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// no Images, no AllowedValues → full exemption
			},
		}

		p := &Policy{
			exceptions: []compiler.Exception{
				// partial exception is matched first
				{MatchConditions: []cel.Program{}, Exception: partialEx},
				// full exemption is matched second – must still win
				{MatchConditions: []cel.Program{}, Exception: fullEx},
			},
		}

		result, err := p.evaluateWithData(context.Background(), evaluationData{})

		assert.NoError(t, err)
		// The full exemption breaks the loop (resetting any prior partial scopes),
		// and the post-loop check returns with exceptions; no validation is run.
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Exceptions)
		assert.False(t, result.Result, "validation should not have run")
	})

	t.Run("full-exemption takes precedence when appearing first", func(t *testing.T) {
		partialEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}
		fullEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// no Images, no AllowedValues → full exemption
			},
		}

		p := &Policy{
			exceptions: []compiler.Exception{
				// full exemption is matched first
				{MatchConditions: []cel.Program{}, Exception: fullEx},
				// partial exception is matched second
				{MatchConditions: []cel.Program{}, Exception: partialEx},
			},
		}

		result, err := p.evaluateWithData(context.Background(), evaluationData{})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Exceptions)
		assert.False(t, result.Result, "validation should not have run")
	})

	t.Run("partial exception alone does not skip evaluation", func(t *testing.T) {
		partialEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}

		// A single validation that always returns true.
		alwaysPass := &mockVpolProgram{retVal: types.Bool(true)}

		p := &Policy{
			exceptions: []compiler.Exception{
				{MatchConditions: []cel.Program{}, Exception: partialEx},
			},
			validations: []compiler.Validation{
				{Program: alwaysPass},
			},
		}

		result, err := p.evaluateWithData(context.Background(), evaluationData{})

		// A partial exception alone must NOT skip validation; the policy is evaluated.
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Result, "validation should have run and passed")
	})

	t.Run("all matched exceptions collected when priority labels and reportResult differ", func(t *testing.T) {
		// Regression guard for the exhaustive-loop requirement from the maintainer review:
		// matchedExceptions must be complete so the engine can (a) pick the
		// highest-priority exception via polex.kyverno.io/priority and (b) build
		// the user-facing message that lists every matched exception key.
		//
		// highPriorityEx has priority=10 (the winner for report selection).
		// laterEx has a lower priority but carries reportResult: pass, which
		// would silently override the skip result if the engine only saw *it*.
		// With the old break-based loop the second exception was never collected;
		// with the flag-based loop both must appear in result.Exceptions.
		highPriorityEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// full exemption – no Images, no AllowedValues
			},
		}
		highPriorityEx.SetLabels(map[string]string{
			"polex.kyverno.io/priority": "10",
		})

		laterEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// full exemption as well; carries reportResult: pass
				ReportResult: "pass",
			},
		}
		laterEx.SetLabels(map[string]string{
			"polex.kyverno.io/priority": "5",
		})

		p := &Policy{
			exceptions: []compiler.Exception{
				// high-priority exception is first in iteration order
				{MatchConditions: []cel.Program{}, Exception: highPriorityEx},
				// lower-priority exception with reportResult: pass comes second
				{MatchConditions: []cel.Program{}, Exception: laterEx},
			},
		}

		result, err := p.evaluateWithData(context.Background(), evaluationData{})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Both exceptions must be present so the engine sees the complete set.
		assert.Len(t, result.Exceptions, 2, "both exceptions must be collected by the exhaustive loop")
	})
}
