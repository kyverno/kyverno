package compiler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	cel2 "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	cel1 "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"

	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

// mock Context
type fakeContext struct{}

func (f *fakeContext) GenerateResources(string, []map[string]any) error        { return nil }
func (f *fakeContext) GetGlobalReference(name, projection string) (any, error) { return name, nil }
func (f *fakeContext) GetImageData(image string) (map[string]any, error) {
	return map[string]any{"test": image}, nil
}
func (f *fakeContext) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}
func (f *fakeContext) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{}, nil
}
func (f *fakeContext) GetGeneratedResources() []*unstructured.Unstructured { return nil }
func (f *fakeContext) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}
func (f *fakeContext) ClearGeneratedResources() {}
func (f *fakeContext) SetGenerateContext(polName, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache bool) {
	panic("not implemented")
}

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

func TestVariables(t *testing.T) {
	t.Run("return lazyMap successfully", func(t *testing.T) {
		ctx := context.TODO()

		env, _ := cel2.NewEnv()
		adapter := env.CELTypeAdapter()
		value := types.NewStringStringMap(adapter, map[string]string{"foo": "bar"})

		declType := apiservercel.NewSimpleTypeWithMinSize(
			"map<string, dyn>",
			celtypes.NewMapType(celtypes.StringType, celtypes.DynType),
			value,
			0,
		)

		prog := &mockProgram{retVal: types.String("value")}
		evaluator := &mutating.PolicyEvaluator{
			CompositionEnv: &cel1.CompositionEnv{
				MapType: declType,
				CompiledVariables: map[string]cel1.CompilationResult{
					"foo": {
						Program: prog,
					},
				},
			},
		}

		cctx := &compositionContext{
			ctx:             ctx,
			evaluator:       evaluator,
			contextProvider: &fakeContext{},
		}

		act := &fakeActivation{
			vars: map[string]interface{}{
				"object":    map[string]interface{}{"name": "obj"},
				"oldObject": map[string]interface{}{"name": "old"},
			},
		}

		val := cctx.Variables(act)
		assert.NotNil(t, val)

		// Access via traits.Mapper interface
		mapVal, ok := val.(traits.Mapper)
		assert.True(t, ok, "expected val to implement traits.Mapper")

		fooVal := mapVal.Get(types.String("foo"))
		assert.Equal(t, types.String("value"), fooVal)
	})

	t.Run("returns error", func(t *testing.T) {
		ctx := context.TODO()

		env, _ := cel2.NewEnv()
		adapter := env.CELTypeAdapter()
		value := types.NewStringStringMap(adapter, map[string]string{})

		declType := apiservercel.NewSimpleTypeWithMinSize(
			"map<string, dyn>",
			celtypes.NewMapType(celtypes.StringType, celtypes.DynType),
			value,
			0,
		)

		mockErr := fmt.Errorf("simulated error")
		prog := &mockProgram{
			retVal: nil,
			err:    mockErr,
		}

		evaluator := &mutating.PolicyEvaluator{
			CompositionEnv: &cel1.CompositionEnv{
				MapType: declType,
				CompiledVariables: map[string]cel1.CompilationResult{
					"errorVar": {
						Program: prog,
					},
				},
			},
		}

		cctx := &compositionContext{
			ctx:             ctx,
			evaluator:       evaluator,
			contextProvider: &fakeContext{},
		}

		act := &fakeActivation{
			vars: map[string]interface{}{
				"object":    map[string]interface{}{"name": "obj"},
				"oldObject": map[string]interface{}{"name": "old"},
			},
		}

		val := cctx.Variables(act)
		assert.NotNil(t, val)

		mapVal, ok := val.(traits.Mapper)
		assert.True(t, ok, "expected val to implement traits.Mapper")

		result := mapVal.Get(types.String("errorVar"))
		assert.NotNil(t, result)

		// Should be a wrapped error
		_, isErr := result.(*types.Err)
		assert.True(t, isErr, "expected result to be *types.Err, got %T", result)
	})

}

type fakeActivation struct {
	vars map[string]interface{}
}

func (f *fakeActivation) ResolveName(name string) (interface{}, bool) {
	v, ok := f.vars[name]
	return v, ok
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

type mockAttributes struct{}

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
	t.Run("error in matcher", func(t *testing.T) {
		p := &Policy{
			evaluator: mutating.PolicyEvaluator{
				Matcher: &fakeMatcher{
					err: errors.New("match error"),
				},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &fakeContext{})
		assert.NotNil(t, res)
		assert.EqualError(t, res.Error, "match error")
	})

	t.Run("no match", func(t *testing.T) {
		p := &Policy{
			evaluator: mutating.PolicyEvaluator{
				Matcher: &fakeMatcher{
					matches: false,
				},
			},
		}

		resp := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &fakeContext{})
		assert.Nil(t, resp)
	})

	t.Run("patch error", func(t *testing.T) {
		p := Policy{
			evaluator: mutating.PolicyEvaluator{
				Matcher: &fakeMatcher{
					matches: true,
				},
				Mutators: []patch.Patcher{
					&fakePatcher{
						retVal: nil,
						err:    errors.New("patch failed"),
					},
				},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &fakeContext{})
		assert.NotNil(t, res)
		assert.EqualError(t, res.Error, "patch failed")
	})

	t.Run("successful evaluation", func(t *testing.T) {
		patchedObj := &unstructured.Unstructured{}
		p := &Policy{
			evaluator: mutating.PolicyEvaluator{
				Matcher: &fakeMatcher{
					matches: true,
				},
				Mutators: []patch.Patcher{
					&fakePatcher{
						retVal: patchedObj,
						err:    nil,
					},
				},
			},
		}

		res := p.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, &fakeContext{})
		assert.NotNil(t, res)
		assert.Equal(t, patchedObj, res.PatchedResource)
	})
}

func TestMatchesConditions(t *testing.T) {
	ctx := context.TODO()
	t.Run("no matcher", func(t *testing.T) {
		p := Policy{}
		res := p.MatchesConditions(ctx, &mockAttributes{}, &corev1.Namespace{})
		assert.False(t, res)
	})

	t.Run("with error", func(t *testing.T) {
		p := Policy{
			evaluator: mutating.PolicyEvaluator{
				Matcher: &fakeMatcher{
					err: errors.New("match error"),
				},
			},
		}

		res := p.MatchesConditions(ctx, &mockAttributes{}, &corev1.Namespace{})
		assert.False(t, res)
	})

	t.Run("no match", func(t *testing.T) {
		p := Policy{
			evaluator: mutating.PolicyEvaluator{
				Matcher: &fakeMatcher{matches: false},
			},
		}
		res := p.MatchesConditions(ctx, &mockAttributes{}, &corev1.Namespace{})
		assert.False(t, res)
	})

	t.Run("match successfully", func(t *testing.T) {
		p := Policy{
			evaluator: mutating.PolicyEvaluator{
				Matcher: &fakeMatcher{matches: true},
			},
		}
		res := p.MatchesConditions(ctx, &mockAttributes{}, &corev1.Namespace{})
		assert.True(t, res)
	})
}
