package libs

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/version"
)

// staticEntry is a minimal store.Entry that always returns the same
// projection-agnostic value, mirroring what a GlobalContextEntry apiCall
// backend stores after fetching a Kubernetes list response.
type staticEntry struct {
	data any
}

func (e *staticEntry) Get(projection string) (any, error) {
	return e.data, nil
}

func (e *staticEntry) Stop() {}

// TestContextProvider_GetGlobalReference_KubernetesShapedEntry reproduces the
// deterministic conformance failure at
// test/conformance/chainsaw/deleting-policies/cel-lib/globalcontext-lib: a
// DeletingPolicy condition calling globalContext.get(entry, "") against a
// GlobalContextEntry whose apiCall response is a Kubernetes list (has
// apiVersion+kind, so contextProvider.GetGlobalReference treats it as a
// Kubernetes object and converts it via kubeutils.ObjToUnstructured) fails at
// CEL evaluation time since the cel-go v0.30.0 -> v0.31.0 bump (#17067):
// GetGlobalReference (context.go) hands the sdk globalcontext lib's
// NativeToValue a bare, unregistered unstructured.Unstructured value, and
// v0.31.0 (unlike v0.30.0) requires native struct types to be explicitly
// registered before NativeToValue can convert them.
func TestContextProvider_GetGlobalReference_KubernetesShapedEntry(t *testing.T) {
	gctxStore := store.New(0)
	// Mirrors the real fixture's GlobalContextEntry apiCall response: a
	// DeploymentList (has apiVersion+kind -> isLikelyKubernetesObject == true).
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

	// Build the same kind of CEL env the dpol/vpol/mpol/gpol/ivpol compilers
	// build when they register the globalcontext library (see e.g.
	// pkg/cel/policies/dpol/compiler/compiler.go createBaseDpolEnv).
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
