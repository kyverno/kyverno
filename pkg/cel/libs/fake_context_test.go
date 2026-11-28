package libs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewFakeContextProvider(t *testing.T) {
	cp := NewFakeContextProvider()
	assert.Equal(
		t, &FakeContextProvider{
			resources:        map[string]map[string]map[string]*unstructured.Unstructured{},
			images:           map[string]map[string]any{},
			globalReferences: map[string]any{},
		},
		cp,
	)
}

func TestFakeContextProvider_GetGlobalReference(t *testing.T) {
	cp := &FakeContextProvider{}
	v, err := cp.GetGlobalReference("foo", "bar")
	assert.NoError(t, err)
	assert.Nil(t, v)

	cp2 := NewFakeContextProvider()
	v, err = cp2.GetGlobalReference("missing", "")
	assert.NoError(t, err)
	assert.Nil(t, v)

	cp2.AddGlobalReference("key", map[string]any{"hello": "world"})
	val, err := cp2.GetGlobalReference("key", "")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"hello": "world"}, val)

	// Projection lookup: returns only the requested projection value
	cp3 := NewFakeContextProvider()
	cp3.AddGlobalReference("entry", map[string]any{
		"names": []interface{}{"dep-1", "dep-2"},
		"count": float64(2),
	})
	projVal, err := cp3.GetGlobalReference("entry", "names")
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"dep-1", "dep-2"}, projVal)

	projVal2, err := cp3.GetGlobalReference("entry", "count")
	assert.NoError(t, err)
	assert.Equal(t, float64(2), projVal2)

	// Missing projection returns error
	_, err = cp3.GetGlobalReference("entry", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "projection")

	// Projection on non-map data returns error
	cp4 := NewFakeContextProvider()
	cp4.AddGlobalReference("list-entry", []interface{}{"a", "b"})
	_, err = cp4.GetGlobalReference("list-entry", "proj")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an object")
}

func TestFakeContextProvider_AddGlobalReference_initializesNilMap(t *testing.T) {
	cp := &FakeContextProvider{}
	cp.AddGlobalReference("k", map[string]any{"v": true})
	got, err := cp.GetGlobalReference("k", "")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"v": true}, got)
}

func TestFakeContextProvider_AddImageData(t *testing.T) {
	cp := NewFakeContextProvider()

	imageName := "nginx:latest"
	imageData := map[string]any{
		"image":    "nginx:latest",
		"registry": "index.docker.io",
		"digest":   "sha256:12345",
		"manifest": map[string]any{
			"annotations": map[string]any{
				"org.opencontainers.image.base.name": "debian:latest",
			},
		},
	}
	cp.AddImageData(imageName, imageData)
	got, err := cp.GetImageData(imageName, nil)
	assert.NoError(t, err)
	assert.Equal(t, imageData, got)
	got, err = cp.GetImageData("missing:latest", nil)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestFakeContextProvider_PostResource(t *testing.T) {
	cp := &FakeContextProvider{}
	assert.Panics(t, func() { cp.PostResource("v1", "configmaps", "default", nil) })
}

func TestFakeContextProvider_AddResource(t *testing.T) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-name",
		},
	}
	want, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	assert.NoError(t, err)
	cp := NewFakeContextProvider()
	assert.NoError(
		t,
		cp.AddResource(
			schema.GroupVersionResource{
				Version:  "v1",
				Resource: "configmaps",
			},
			cm,
		),
	)
	// get
	{
		got, err := cp.GetResource("v1", "configmaps", "test-ns", "test-name")
		assert.NoError(t, err)
		assert.Equal(t, want, got.Object)
	}
	{
		got, err := cp.GetResource("v1", "configmaps", "test-ns", "wrong-name")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.GetResource("v1", "configmaps", "wrong-ns", "test-name")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.GetResource("v1", "wrongs", "test-ns", "test-name")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.GetResource("wrong", "configmaps", "test-ns", "test-name")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	// list
	{
		got, err := cp.ListResources("v1", "configmaps", "test-ns", nil)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(got.Items))
	}
	{
		got, err := cp.ListResources("v1", "configmaps", "wrong-ns", nil)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(got.Items))
	}
	{
		got, err := cp.ListResources("v1", "wrongs", "test-ns", nil)
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.ListResources("wrong", "configmaps", "test-ns", nil)
		assert.Error(t, err)
		assert.Nil(t, got)
	}
}

func TestFakeContextProvider_Clone(t *testing.T) {
	original := &FakeContextProvider{
		generatedResources: make([]*unstructured.Unstructured, 0),
	}
	original.SetGenerateContext("policyA", "default", "triggerA", "default", "v1", "", "Pod", "123", true, false)

	// Pre-populate original to verify Clone() initializes an isolated, empty slice
	originalResource := &unstructured.Unstructured{Object: map[string]interface{}{"kind": "ConfigMap"}}
	original.generatedResources = append(original.generatedResources, originalResource)

	cloneContext := original.Clone()
	clone, ok := cloneContext.(*FakeContextProvider)
	assert.True(t, ok, "cloned context should be of type *FakeContextProvider")

	clone.SetGenerateContext("policyB", "kube-system", "triggerB", "kube-system", "v1", "", "Service", "456", false, false)

	assert.Equal(t, "policyA", original.policyName)
	assert.Equal(t, true, original.restoreCache)

	assert.Equal(t, "policyB", clone.policyName)
	assert.Equal(t, false, clone.restoreCache)

	// Verify slice isolation
	assert.Len(t, original.generatedResources, 1)
	assert.Len(t, clone.generatedResources, 0)

	// Verify mutations don't leak to the original provider
	mockResource := &unstructured.Unstructured{Object: map[string]interface{}{"kind": "Pod"}}
	clone.generatedResources = append(clone.generatedResources, mockResource)

	assert.Len(t, original.generatedResources, 1)
	assert.Len(t, clone.generatedResources, 1)
}

func TestFakeContextProvider_ListResources(t *testing.T) {
	cp := NewFakeContextProvider()
	cmGVR := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	crGVR := schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}

	require.NoError(t, cp.AddResource(cmGVR, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-a",
			Namespace: "ns-a",
			Labels:    map[string]string{"app": "web", "tier": "frontend"},
		},
	}))
	require.NoError(t, cp.AddResource(cmGVR, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-a",
			Namespace: "ns-a",
			Labels:    map[string]string{"app": "db"},
		},
	}))
	require.NoError(t, cp.AddResource(cmGVR, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-b",
			Namespace: "ns-b",
			Labels:    map[string]string{"app": "web"},
		},
	}))
	require.NoError(t, cp.AddResource(cmGVR, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unlabeled-b",
			Namespace: "ns-b",
		},
	}))

	require.NoError(t, cp.AddResource(crGVR, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "role-a",
			Labels: map[string]string{"rbac": "admin"},
		},
	}))
	require.NoError(t, cp.AddResource(crGVR, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "role-b",
		},
	}))

	itemIDs := func(list *unstructured.UnstructuredList) []string {
		ids := make([]string, 0, len(list.Items))
		for _, item := range list.Items {
			if item.GetNamespace() != "" {
				ids = append(ids, item.GetNamespace()+"/"+item.GetName())
			} else {
				ids = append(ids, item.GetName())
			}
		}
		return ids
	}

	tests := []struct {
		name       string
		apiVersion string
		resource   string
		namespace  string
		labels     map[string]string
		wantIDs    []string
		wantErr    bool
	}{
		{
			name:       "namespace-specific without selector",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "ns-a",
			labels:     nil,
			wantIDs:    []string{"ns-a/db-a", "ns-a/web-a"},
		},
		{
			name:       "all-namespaces with nil selector includes unlabeled",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "",
			labels:     nil,
			wantIDs:    []string{"ns-a/db-a", "ns-a/web-a", "ns-b/unlabeled-b", "ns-b/web-b"},
		},
		{
			name:       "all-namespaces with empty map selector",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "",
			labels:     map[string]string{},
			wantIDs:    []string{"ns-a/db-a", "ns-a/web-a", "ns-b/unlabeled-b", "ns-b/web-b"},
		},
		{
			name:       "namespace-specific with matching selector",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "ns-a",
			labels:     map[string]string{"app": "web"},
			wantIDs:    []string{"ns-a/web-a"},
		},
		{
			name:       "all-namespaces with matching selector excludes unlabeled",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "",
			labels:     map[string]string{"app": "web"},
			wantIDs:    []string{"ns-a/web-a", "ns-b/web-b"},
		},
		{
			name:       "namespace-specific with multi-label exact match",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "ns-a",
			labels:     map[string]string{"app": "web", "tier": "frontend"},
			wantIDs:    []string{"ns-a/web-a"},
		},
		{
			name:       "unmatched selector in namespace returns empty list",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "ns-a",
			labels:     map[string]string{"app": "none"},
			wantIDs:    []string{},
		},
		{
			name:       "unmatched selector across all namespaces returns empty list",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "",
			labels:     map[string]string{"app": "none"},
			wantIDs:    []string{},
		},
		{
			name:       "unmatched namespace returns empty list",
			apiVersion: "v1",
			resource:   "configmaps",
			namespace:  "ns-missing",
			labels:     nil,
			wantIDs:    []string{},
		},
		{
			name:       "cluster-scoped resources in empty namespace without selector",
			apiVersion: "rbac.authorization.k8s.io/v1",
			resource:   "clusterroles",
			namespace:  "",
			labels:     nil,
			wantIDs:    []string{"role-a", "role-b"},
		},
		{
			name:       "cluster-scoped resources in empty namespace with selector",
			apiVersion: "rbac.authorization.k8s.io/v1",
			resource:   "clusterroles",
			namespace:  "",
			labels:     map[string]string{"rbac": "admin"},
			wantIDs:    []string{"role-a"},
		},
		{
			name:       "unknown resource returns error",
			apiVersion: "v1",
			resource:   "wrongs",
			namespace:  "ns-a",
			wantErr:    true,
		},
		{
			name:       "unknown apiVersion returns error",
			apiVersion: "wrong",
			resource:   "configmaps",
			namespace:  "ns-a",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cp.ListResources(tt.apiVersion, tt.resource, tt.namespace, tt.labels)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantIDs, itemIDs(got))
		})
	}
}
