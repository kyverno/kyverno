package libs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func generateTestRESTMapper() meta.RESTMapper {
	gvs := []schema.GroupVersion{
		{Group: "", Version: "v1"},
		{Group: "rbac.authorization.k8s.io", Version: "v1"},
		{Group: "networking.k8s.io", Version: "v1"},
	}
	m := meta.NewDefaultRESTMapper(gvs)
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}, meta.RESTScopeRoot)
	m.Add(schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "IngressClass"}, meta.RESTScopeRoot)
	return m
}

func clusterRoleBinding() map[string]any {
	return map[string]any{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "ClusterRoleBinding",
		"metadata":   map[string]any{"name": "escalate"},
	}
}

// A namespaced policy passes its own namespace as the apply() argument, which
// satisfies the namespace-boundary check, but a cluster-scoped resource still
// escapes that namespace. It must be rejected.
func TestGenerateResources_NamespacedPolicyRejectsClusterScoped(t *testing.T) {
	cp := &contextProvider{
		cliEvaluation: true,
		restMapper:    generateTestRESTMapper(),
	}

	err := cp.GenerateResources("tenant-ns", []map[string]any{clusterRoleBinding()})
	assert.Error(t, err, "namespaced policy must not generate a cluster-scoped resource")
	assert.Contains(t, err.Error(), "cross-scope generation denied")
	assert.Empty(t, cp.GetGeneratedResources())
}

// A namespaced resource generated into the policy namespace is still allowed.
func TestGenerateResources_NamespacedPolicyAllowsNamespaced(t *testing.T) {
	cp := &contextProvider{
		cliEvaluation: true,
		restMapper:    generateTestRESTMapper(),
	}

	cm := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "data", "namespace": "tenant-ns"},
	}
	err := cp.GenerateResources("tenant-ns", []map[string]any{cm})
	assert.NoError(t, err)
	assert.Len(t, cp.GetGeneratedResources(), 1)
	assert.Equal(t, "tenant-ns", cp.GetGeneratedResources()[0].GetNamespace())
}

// A cluster-scoped policy uses an empty namespace argument and may still
// generate cluster-scoped resources.
func TestGenerateResources_ClusterPolicyAllowsClusterScoped(t *testing.T) {
	cp := &contextProvider{
		cliEvaluation: true,
		restMapper:    generateTestRESTMapper(),
	}

	err := cp.GenerateResources("", []map[string]any{clusterRoleBinding()})
	assert.NoError(t, err)
	assert.Len(t, cp.GetGeneratedResources(), 1)
	assert.Empty(t, cp.GetGeneratedResources()[0].GetNamespace())
}
