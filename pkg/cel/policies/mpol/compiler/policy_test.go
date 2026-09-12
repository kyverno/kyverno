package compiler

import (
	"context"
	"errors"
	"testing"
	"time"

	cel2 "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type mockProgram struct {
	retVal ref.Val
	err    error
}

func (m *mockProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel2.EvalDetails, error) {
	return m.retVal, nil, m.err
}

func (m *mockProgram) Eval(any) (ref.Val, *cel2.EvalDetails, error) {
	return m.retVal, nil, m.err
}

// FakeContextWithDeadline provides context with deadline and cancel
func FakeContextWithDeadline(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

type fakeMatcher struct {
	err     error
	matches bool
}

func (f *fakeMatcher) Match(ctx context.Context, versionedAttr *admission.VersionedAttributes, versionedParams runtime.Object, authz authorizer.Authorizer) matchconditions.MatchResult {
	return matchconditions.MatchResult{
		Matches: f.matches,
		Error:   f.err,
	}
}

type fakeTCM struct{}

func (f *fakeTCM) GetTypeConverter(_ schema.GroupVersionKind) managedfields.TypeConverter {
	return managedfields.NewDeducedTypeConverter()
}

type mockAttributes struct {
	obj    runtime.Object
	oldObj runtime.Object
}

func (m *mockAttributes) GetName() string      { return "" }
func (m *mockAttributes) GetNamespace() string { return "default" }
func (m *mockAttributes) GetResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
}
func (m *mockAttributes) GetSubresource() string { return "" }
func (m *mockAttributes) GetOperation() admission.Operation {
	return admission.Create
}
func (m *mockAttributes) GetOperationOptions() runtime.Object { return nil }
func (m *mockAttributes) IsDryRun() bool                      { return false }
func (m *mockAttributes) GetObject() runtime.Object {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
			Labels:    map[string]string{},
		},
	}
}
func (m *mockAttributes) GetOldObject() runtime.Object { return nil }
func (m *mockAttributes) GetKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
}
func (m *mockAttributes) GetUserInfo() user.Info                { return &user.DefaultInfo{} }
func (m *mockAttributes) AddAnnotation(key, value string) error { return nil }
func (m *mockAttributes) AddAnnotationWithLevel(key, value string, level auditinternal.Level) error {
	return nil
}
func (m *mockAttributes) GetReinvocationContext() admission.ReinvocationContext { return nil }

type fakePatcher struct {
	retVal runtime.Object
	err    error
}

func (f *fakePatcher) Patch(ctx context.Context, evalData map[string]any, patchRequest patch.Request, runtimeCELCostBudget int64) (runtime.Object, error) {
	return f.retVal, f.err
}

func TestEvaluate(t *testing.T) {
	ctx := context.TODO()
	t.Run("patch error", func(t *testing.T) {
		p := Policy{
			patchers: []Patcher{
				&fakePatcher{
					retVal: nil,
					err:    errors.New("patch failed"),
				},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &libs.FakeContextProvider{})
		assert.NotNil(t, res)
		assert.EqualError(t, res.Error, "patch failed")
	})

	t.Run("successful evaluation", func(t *testing.T) {
		patchedObj := &unstructured.Unstructured{}
		p := &Policy{
			patchers: []Patcher{
				&fakePatcher{
					retVal: patchedObj,
					err:    nil,
				},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &libs.FakeContextProvider{})
		assert.NotNil(t, res)
		assert.Equal(t, patchedObj, res.PatchedResource)
	})

	t.Run("full-exemption exception takes precedence over partial exceptions", func(t *testing.T) {
		// Regression test for https://github.com/kyverno/kyverno/issues/16053:
		// When both a partial exception (with Images) and a full-exemption exception
		// (no Images, no AllowedValues) match, the full exemption must win and the
		// policy must be skipped.
		partialEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}
		fullEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// empty → full exemption
			},
		}
		p := &Policy{
			exceptions: []compiler.Exception{
				{MatchConditions: []cel2.Program{}, Exception: partialEx},
				{MatchConditions: []cel2.Program{}, Exception: fullEx},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &libs.FakeContextProvider{})

		assert.NotNil(t, res)
		assert.Nil(t, res.Error)
		assert.Nil(t, res.PatchedResource)
		assert.NotEmpty(t, res.Exceptions)
	})

	t.Run("full-exemption exception takes precedence over partial exceptions (reversed order)", func(t *testing.T) {
		partialEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}
		fullEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// empty → full exemption
			},
		}
		p := &Policy{
			exceptions: []compiler.Exception{
				{MatchConditions: []cel2.Program{}, Exception: fullEx},
				{MatchConditions: []cel2.Program{}, Exception: partialEx},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &libs.FakeContextProvider{})

		assert.NotNil(t, res)
		assert.Nil(t, res.Error)
		assert.Nil(t, res.PatchedResource)
		assert.NotEmpty(t, res.Exceptions)
	})

	t.Run("successful evaluation with audit annotations", func(t *testing.T) {
		patchedObj := &unstructured.Unstructured{}
		p := &Policy{
			patchers: []Patcher{
				&fakePatcher{
					retVal: patchedObj,
					err:    nil,
				},
			},
			auditAnnotations: map[string]cel2.Program{
				"resource-name": &fakeProgram{refVal: types.String("nginx")},
				"empty":         &fakeProgram{refVal: types.String("")},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &libs.FakeContextProvider{})
		assert.NotNil(t, res)
		assert.Equal(t, patchedObj, res.PatchedResource)
		assert.Equal(t, map[string]string{"resource-name": "nginx"}, res.AuditAnnotations)
	})

	t.Run("audit annotation evaluation error", func(t *testing.T) {
		p := &Policy{
			patchers: []Patcher{
				&fakePatcher{
					retVal: &unstructured.Unstructured{},
					err:    nil,
				},
			},
			auditAnnotations: map[string]cel2.Program{
				"bad": &fakeProgram{err: errors.New("eval failed")},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &libs.FakeContextProvider{})
		assert.NotNil(t, res)
		assert.ErrorContains(t, res.Error, "failed to evaluate auditAnnotation \"bad\"")
	})
}

type fakeProgram struct {
	refVal ref.Val
	err    error
}

func (f *fakeProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel2.EvalDetails, error) {
	return f.refVal, nil, f.err
}

func (f *fakeProgram) Eval(_ any) (ref.Val, *cel2.EvalDetails, error) {
	return f.refVal, nil, nil
}
func (f *fakeProgram) ConcurrentEval(_ context.Context, _ any) <-chan cel2.EvalResult { return nil }
