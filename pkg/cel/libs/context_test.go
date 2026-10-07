package libs

import (
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/version"
)

// staticEntry is a store.Entry that always returns the same value.
type staticEntry struct {
	data any
}

func (e *staticEntry) Get(projection string) (any, error) {
	return e.data, nil
}

func (e *staticEntry) Stop() {}

// TestContextProvider_GetGlobalReference_KubernetesShapedEntry covers globalContext.get on an
// entry holding a Kubernetes list, which failed to convert to a CEL value after cel-go v0.31.
func TestContextProvider_GetGlobalReference_KubernetesShapedEntry(t *testing.T) {
	gctxStore := store.New(0)
	// Same shape as the deleting-policies/cel-lib/globalcontext-lib conformance fixture.
	require.NoError(t, gctxStore.Set("gctxentry-apicall-correct", &staticEntry{
		data: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "DeploymentList",
			"items": []any{
				map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "test-globalcontext-apicall-correct",
					},
				},
			},
		},
	}))

	cp := &contextProvider{gctxStore: gctxStore}

	// Same env the policy compilers build for the globalcontext library.
	env, err := cel.NewEnv(
		globalcontext.Lib(
			globalcontext.Context{ContextInterface: cp},
			version.MajorMinor(1, 18),
		),
	)
	require.NoError(t, err)

	ast, iss := env.Compile(`globalContext.get("gctxentry-apicall-correct", "") != 0`)
	require.NoError(t, iss.Err(), "policy expression must compile")

	prg, err := env.Program(ast)
	require.NoError(t, err)

	_, _, err = prg.Eval(map[string]any{
		"globalContext": globalcontext.Context{ContextInterface: cp},
	})
	require.NoError(t, err, "globalContext.get on a Kubernetes-shaped entry must not fail at evaluation time")
}

// failingEntry is a store.Entry whose Get always fails.
type failingEntry struct{}

func (e *failingEntry) Get(projection string) (any, error) {
	return nil, errors.New("api call failed")
}

func (e *failingEntry) Stop() {}

func TestContextProvider_GetGlobalReference(t *testing.T) {
	gctxStore := store.New(0)
	require.NoError(t, gctxStore.Set("failing", &failingEntry{}))
	require.NoError(t, gctxStore.Set("scalar", &staticEntry{data: "value"}))
	require.NoError(t, gctxStore.Set("kube-object", &staticEntry{data: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm"},
	}}))
	cp := &contextProvider{gctxStore: gctxStore}

	tests := []struct {
		name    string
		entry   string
		want    any
		wantErr string
	}{{
		name:  "an absent entry gives no value and no error",
		entry: "missing",
		want:  nil,
	}, {
		name:    "an entry that fails returns its error",
		entry:   "failing",
		wantErr: "api call failed",
	}, {
		name:  "a value that is not a Kubernetes object is returned as is",
		entry: "scalar",
		want:  "value",
	}, {
		name:  "a Kubernetes object is returned as a map",
		entry: "kube-object",
		want: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "cm"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cp.GetGlobalReference(tt.entry, "")
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// An error from the entry must fail the CEL evaluation instead of yielding a value.
func TestContextProvider_GetGlobalReference_EntryErrorFailsEvaluation(t *testing.T) {
	gctxStore := store.New(0)
	require.NoError(t, gctxStore.Set("failing", &failingEntry{}))
	cp := &contextProvider{gctxStore: gctxStore}
	env, err := cel.NewEnv(
		globalcontext.Lib(
			globalcontext.Context{ContextInterface: cp},
			version.MajorMinor(1, 18),
		),
	)
	require.NoError(t, err)
	ast, iss := env.Compile(`globalContext.get("failing", "") != 0`)
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast)
	require.NoError(t, err)
	_, _, err = prg.Eval(map[string]any{
		"globalContext": globalcontext.Context{ContextInterface: cp},
	})
	require.ErrorContains(t, err, "api call failed")
}
