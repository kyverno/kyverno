package compiler

import (
	"context"
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type valueProgram struct{}

func (v *valueProgram) Eval(_ any) (ref.Val, *cel.EvalDetails, error) { return nil, nil, nil }
func (v *valueProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return types.String("test"), nil, nil
}

type errorProgram struct{}

func (e *errorProgram) Eval(_ any) (ref.Val, *cel.EvalDetails, error) { return nil, nil, nil }
func (e *errorProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return nil, nil, errors.New("forced variable error")
}

type trueProgram struct{}

func (t *trueProgram) Eval(_ any) (ref.Val, *cel.EvalDetails, error) {
	return types.Bool(true), nil, nil
}
func (t *trueProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return types.Bool(true), nil, nil
}

type falseProgram struct{}

func (f *falseProgram) Eval(_ any) (ref.Val, *cel.EvalDetails, error) {
	return types.Bool(false), nil, nil
}
func (f *falseProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return types.Bool(false), nil, nil
}

type evalErrorProgram struct{}

func (e *evalErrorProgram) Eval(_ any) (ref.Val, *cel.EvalDetails, error) {
	return nil, nil, errors.New("forced match error")
}
func (e *evalErrorProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return nil, nil, errors.New("forced match error")
}

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

func TestEvaluate(t *testing.T) {
	ctx := context.Background()
	obj := unstructured.Unstructured{}
	ctxLib := &fakeContext{}

	t.Run("variable returns value", func(t *testing.T) {
		p := &Policy{variables: map[string]cel.Program{"test": &valueProgram{}}}
		result, err := p.Evaluate(ctx, obj, ctxLib)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("variable returns error", func(t *testing.T) {
		p := &Policy{variables: map[string]cel.Program{"test": &errorProgram{}}}
		result, err := p.Evaluate(ctx, obj, ctxLib)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("match returns true (conditions)", func(t *testing.T) {
		p := &Policy{conditions: []cel.Program{&trueProgram{}}}
		result, err := p.Evaluate(ctx, obj, ctxLib)
		require.NoError(t, err)
		require.True(t, result.Result)
	})

	t.Run("match returns false (conditions)", func(t *testing.T) {
		p := &Policy{conditions: []cel.Program{&falseProgram{}}}
		result, err := p.Evaluate(ctx, obj, ctxLib)
		require.NoError(t, err)
		require.False(t, result.Result)
	})

	t.Run("match returns error (conditions)", func(t *testing.T) {
		p := &Policy{conditions: []cel.Program{&evalErrorProgram{}}}
		result, err := p.Evaluate(ctx, obj, ctxLib)
		require.Error(t, err)
		require.Nil(t, result)
	})

	t.Run("exception match returns true", func(t *testing.T) {
		p := &Policy{
			exceptions: []compiler.Exception{{
				Exception:       &policiesv1alpha1.PolicyException{},
				MatchConditions: []cel.Program{&trueProgram{}},
			}},
		}
		result, err := p.Evaluate(ctx, obj, ctxLib)
		require.NoError(t, err)
		require.Len(t, result.Exceptions, 1)
	})

	t.Run("exception match returns false", func(t *testing.T) {
		p := &Policy{
			exceptions: []compiler.Exception{{
				Exception:       &policiesv1alpha1.PolicyException{},
				MatchConditions: []cel.Program{&falseProgram{}},
			}},
		}
		result, err := p.Evaluate(ctx, obj, ctxLib)
		require.NoError(t, err)
		require.Empty(t, result.Exceptions)
	})

	t.Run("exception match returns error", func(t *testing.T) {
		p := &Policy{
			exceptions: []compiler.Exception{{
				Exception:       &policiesv1alpha1.PolicyException{},
				MatchConditions: []cel.Program{&evalErrorProgram{}},
			}},
		}
		result, err := p.Evaluate(ctx, obj, ctxLib)
		require.Error(t, err)
		require.Nil(t, result)
	})
}

// mock Programs

type boolProgram struct {
	result bool
}

func (b *boolProgram) Eval(_ any) (ref.Val, *cel.EvalDetails, error) {
	return nil, nil, nil
}
func (b *boolProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return types.Bool(b.result), nil, nil
}

type errorEvalProgram struct{}

func (e *errorEvalProgram) Eval(_ any) (ref.Val, *cel.EvalDetails, error) {
	return nil, nil, nil
}
func (e *errorEvalProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return nil, nil, errors.New("eval error")
}

type errorConvertProgram struct{}

func (e *errorConvertProgram) Eval(_ any) (ref.Val, *cel.EvalDetails, error) {
	return nil, nil, nil
}
func (e *errorConvertProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return types.String("notBool"), nil, nil
}

func TestMatch(t *testing.T) {
	ctx := context.TODO()
	p := &Policy{}
	data := map[string]any{}

	t.Run("single condition true", func(t *testing.T) {
		result, err := p.match(ctx, data, []cel.Program{&boolProgram{true}})
		require.NoError(t, err)
		require.True(t, result)
	})

	t.Run("single condition false", func(t *testing.T) {
		result, err := p.match(ctx, data, []cel.Program{&boolProgram{false}})
		require.NoError(t, err)
		require.False(t, result)
	})

	t.Run("condition with eval error", func(t *testing.T) {
		result, err := p.match(ctx, data, []cel.Program{&errorEvalProgram{}})
		require.Error(t, err)
		require.False(t, result)
		require.Contains(t, err.Error(), "eval error")
	})

	t.Run("multiple errors combined", func(t *testing.T) {
		conditions := []cel.Program{
			&errorEvalProgram{},
			&errorConvertProgram{},
		}
		result, err := p.match(ctx, data, conditions)
		require.Error(t, err)
		require.False(t, result)
	})
}
