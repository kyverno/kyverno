package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYamlToUnstructured_ClusterScopedResources(t *testing.T) {
	tests := []struct {
		name              string
		yaml              string
		expectedNamespace string
	}{
		{
			name: "Namespace should not get default namespace",
			yaml: `apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace`,
			expectedNamespace: "",
		},
		{
			name: "Pod should get default namespace",
			yaml: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod`,
			expectedNamespace: "default",
		},
		{
			name: "Node should not get default namespace",
			yaml: `apiVersion: v1
kind: Node
metadata:
  name: test-node`,
			expectedNamespace: "",
		},
		{
			name: "ClusterRole should not get default namespace",
			yaml: `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-cluster-role`,
			expectedNamespace: "",
		},
		{
			name: "Deployment should get default namespace",
			yaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment`,
			expectedNamespace: "default",
		},
		{
			name: "PersistentVolume should not get default namespace",
			yaml: `apiVersion: v1
kind: PersistentVolume
metadata:
  name: test-pv`,
			expectedNamespace: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, err := YamlToUnstructured([]byte(tt.yaml))
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedNamespace, resource.GetNamespace(),
				"Resource %s should have namespace %q but got %q",
				resource.GetKind(), tt.expectedNamespace, resource.GetNamespace())
		})
	}
}

func TestIsClusterScopedResource(t *testing.T) {
	tests := []struct {
		kind     string
		expected bool
	}{
		{"Namespace", true},
		{"Node", true},
		{"PersistentVolume", true},
		{"ClusterRole", true},
		{"ClusterRoleBinding", true},
		{"StorageClass", true},
		{"Pod", false},
		{"Deployment", false},
		{"Service", false},
		{"ConfigMap", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := isClusterScopedResource(tt.kind)
			assert.Equal(t, tt.expected, result,
				"Kind %s: expected %v, got %v", tt.kind, tt.expected, result)
		})
	}
}
