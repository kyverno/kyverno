package compiler

import (
	"context"
	"errors"
	"testing"
	"time"

	cel2 "github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
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

func TestGetAndResetCost(t *testing.T) {
	cctx := &compositionContext{
		accumulatedCost: 42,
	}

	// First call should return 42 and reset to 0
	cost := cctx.GetAndResetCost()
	assert.Equal(t, int64(42), cost)
	assert.Equal(t, int64(0), cctx.accumulatedCost)

	// Second call should return 0
	cost = cctx.GetAndResetCost()
	assert.Equal(t, int64(0), cost)
}

func TestDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cctx := &compositionContext{
		ctx: ctx,
	}

	deadline, ok := cctx.Deadline()
	assert.True(t, ok, "Expected Deadline() to return true")
	assert.WithinDuration(t, time.Now().Add(2*time.Second), deadline, time.Second)
}

func TestDoneAndErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cctx := &compositionContext{ctx: ctx}

	// Not cancelled yet
	select {
	case <-cctx.Done():
		t.Fatal("context should not be done")
	default:
		// expected
	}

	// Cancel and test
	cancel()

	select {
	case <-cctx.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("context should be done after cancel")
	}

	assert.Equal(t, context.Canceled, cctx.Err())
}

func TestValue(t *testing.T) {
	key := "testKey"
	value := "testValue"

	ctx := context.WithValue(context.Background(), key, value)
	cctx := &compositionContext{ctx: ctx}

	result := cctx.Value(key)
	assert.Equal(t, value, result)

	// Key not present
	result = cctx.Value("nonexistent")
	assert.Nil(t, result)
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

func (f *fakePatcher) Patch(ctx context.Context, request patch.Request, runtimeCELCostBudget int64) (runtime.Object, error) {
	return f.retVal, f.err
}

func TestEvaluate(t *testing.T) {
	ctx := context.TODO()
	t.Run("patch error", func(t *testing.T) {
		p := Policy{
			evaluator: mutating.PolicyEvaluator{
				Mutators: []patch.Patcher{
					&fakePatcher{
						retVal: nil,
						err:    errors.New("patch failed"),
					},
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
			evaluator: mutating.PolicyEvaluator{
				Mutators: []patch.Patcher{
					&fakePatcher{
						retVal: patchedObj,
						err:    nil,
					},
				},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &libs.FakeContextProvider{})
		assert.NotNil(t, res)
		assert.Equal(t, patchedObj, res.PatchedResource)
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
