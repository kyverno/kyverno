package report

import (
	"testing"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Tests for GVR Serialization & Resource Hashing

func TestSetResourceGVR_GetResourceGVR_RoundTrip_ShortGVR(t *testing.T) {
	// Test round-trip with short GVR (< 63 chars)
	report := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-report",
			Labels: make(map[string]string),
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	SetResourceGVR(report, gvr)
	result := GetResourceGVR(report)

	assert.Equal(t, gvr.Group, result.Group, "Group should match")
	assert.Equal(t, gvr.Version, result.Version, "Version should match")
	assert.Equal(t, gvr.Resource, result.Resource, "Resource should match")
}

func TestSetResourceGVR_GetResourceGVR_RoundTrip_LongGVR(t *testing.T) {
	// Test round-trip with GVR string > 63 chars (split-label fallback)
	report := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-report",
			Labels: make(map[string]string),
		},
	}

	// Create a GVR that exceeds 63 characters total
	gvr := schema.GroupVersionResource{
		Group:    "verylongdomainname.example.com",
		Version:  "v1beta1",
		Resource: "verylongresourcenamethatexceedslimit",
	}

	SetResourceGVR(report, gvr)
	result := GetResourceGVR(report)

	assert.Equal(t, gvr.Group, result.Group, "Group should match for long GVR")
	assert.Equal(t, gvr.Version, result.Version, "Version should match for long GVR")
	assert.Equal(t, gvr.Resource, result.Resource, "Resource should match for long GVR")
}

func TestSetResourceGVR_GetResourceGVR_CoreAPI_EmptyGroup(t *testing.T) {
	// Test with empty group (core API resources like pods, configmaps)
	report := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-report",
			Labels: make(map[string]string),
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "", // Core API has empty group
		Version:  "v1",
		Resource: "pods",
	}

	SetResourceGVR(report, gvr)
	result := GetResourceGVR(report)

	assert.Equal(t, gvr.Group, result.Group, "Empty group should be preserved")
	assert.Equal(t, gvr.Version, result.Version, "Version should match")
	assert.Equal(t, gvr.Resource, result.Resource, "Resource should match")
}

func TestGetResourceGVR_OldCombinedLabelFormat_ThreeParts(t *testing.T) {
	// Test backward compatibility with old combined label (2+ dots)
	report := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-report",
			Labels: map[string]string{
				LabelResourceGVR: "deployments.v1.apps",
			},
		},
	}

	result := GetResourceGVR(report)

	assert.Equal(t, "apps", result.Group)
	assert.Equal(t, "v1", result.Version)
	assert.Equal(t, "deployments", result.Resource)
}

func TestGetResourceGVR_OldCombinedLabelFormat_TwoParts(t *testing.T) {
	// Test with core API format (1 dot) - resource.version
	report := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-report",
			Labels: map[string]string{
				LabelResourceGVR: "pods.v1",
			},
		},
	}

	result := GetResourceGVR(report)

	assert.Equal(t, "", result.Group, "Core API should have empty group")
	assert.Equal(t, "v1", result.Version)
	assert.Equal(t, "pods", result.Resource)
}

func TestGetResourceGVR_SingleResourceString_NoDots(t *testing.T) {
	// Test edge case with single resource string (no dots)
	report := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-report",
			Labels: map[string]string{
				LabelResourceGVR: "pods",
			},
		},
	}

	result := GetResourceGVR(report)

	assert.Equal(t, "", result.Group)
	assert.Equal(t, "", result.Version)
	assert.Equal(t, "pods", result.Resource)
}

func TestCalculateResourceHash_Determinism(t *testing.T) {
	// Same resource should produce same hash
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
				"labels":    map[string]interface{}{"app": "web"},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "nginx", "image": "nginx:latest"},
				},
			},
		},
	}

	hash1 := CalculateResourceHash(resource)
	hash2 := CalculateResourceHash(resource)

	assert.NotEmpty(t, hash1, "Hash should not be empty")
	assert.Equal(t, hash1, hash2, "Same resource should produce same hash")
}

func TestCalculateResourceHash_MetadataStatusIgnored(t *testing.T) {
	// Metadata and status changes should NOT affect hash (they're stripped)
	resource1 := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":            "test-pod",
				"namespace":       "default",
				"resourceVersion": "12345",
				"labels":          map[string]interface{}{"app": "web"},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "nginx", "image": "nginx:latest"},
				},
			},
		},
	}

	resource2 := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":            "test-pod",
				"namespace":       "default",
				"resourceVersion": "67890", // Different resourceVersion
				"labels":          map[string]interface{}{"app": "web"},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "nginx", "image": "nginx:latest"},
				},
			},
			"status": map[string]interface{}{
				"phase": "Running", // Has status field
			},
		},
	}

	hash1 := CalculateResourceHash(resource1)
	hash2 := CalculateResourceHash(resource2)

	assert.Equal(t, hash1, hash2, "Metadata/status changes should not affect hash")
}

func TestCalculateResourceHash_LabelChangesAffectHash(t *testing.T) {
	// Label changes SHOULD affect hash
	resource1 := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
				"labels":    map[string]interface{}{"app": "web"},
			},
			"spec": map[string]interface{}{},
		},
	}

	resource2 := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
				"labels":    map[string]interface{}{"app": "api"}, // Different label value
			},
			"spec": map[string]interface{}{},
		},
	}

	hash1 := CalculateResourceHash(resource1)
	hash2 := CalculateResourceHash(resource2)

	assert.NotEqual(t, hash1, hash2, "Label changes should affect hash")
}

func TestIsPolicyLabel_AllPrefixes(t *testing.T) {
	tests := []struct {
		label    string
		expected bool
	}{
		{LabelPrefixPolicy + "my-policy", true},
		{LabelPrefixClusterPolicy + "my-cpol", true},
		{LabelPrefixValidatingPolicy + "my-vpol", true},
		{LabelPrefixImageValidatingPolicy + "my-ivpol", true},
		{LabelPrefixGeneratingPolicy + "my-gpol", true},
		{LabelPrefixPolicyException + "my-polex", true},
		{LabelPrefixValidatingAdmissionPolicy + "my-vap", true},
		{LabelPrefixValidatingAdmissionPolicyBinding + "my-vapb", true},
		{LabelPrefixMutatingAdmissionPolicy + "my-map", true},
		{LabelPrefixMutatingAdmissionPolicyBinding + "my-mapb", true},
		// Non-policy labels
		{"app", false},
		{"kubernetes.io/name", false},
		{"audit.kyverno.io/resource.hash", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			result := IsPolicyLabel(tt.label)
			assert.Equal(t, tt.expected, result, "IsPolicyLabel(%q) should be %v", tt.label, tt.expected)
		})
	}
}

func TestCleanupKyvernoLabels_MixedLabels(t *testing.T) {
	obj := &metav1.ObjectMeta{
		Labels: map[string]string{
			"app":                            "web",
			"kubernetes.io/name":             "test",
			"cpol.kyverno.io/my-policy":      "v1",
			"audit.kyverno.io/resource.hash": "abc123",
			"team":                           "platform",
		},
	}

	CleanupKyvernoLabels(obj)

	// Kyverno labels should be deleted
	assert.NotContains(t, obj.Labels, "cpol.kyverno.io/my-policy")
	assert.NotContains(t, obj.Labels, "audit.kyverno.io/resource.hash")

	// Non-kyverno labels should remain
	assert.Equal(t, "web", obj.Labels["app"])
	assert.Equal(t, "test", obj.Labels["kubernetes.io/name"])
	assert.Equal(t, "platform", obj.Labels["team"])
}

func TestCleanupKyvernoLabels_EmptyLabels(t *testing.T) {
	obj := &metav1.ObjectMeta{
		Labels: map[string]string{},
	}

	// Should not panic
	CleanupKyvernoLabels(obj)
	assert.Empty(t, obj.Labels)
}

func TestCleanupKyvernoLabels_NilLabels(t *testing.T) {
	obj := &metav1.ObjectMeta{
		Labels: nil,
	}

	// Should not panic
	CleanupKyvernoLabels(obj)
	assert.Nil(t, obj.Labels)
}
