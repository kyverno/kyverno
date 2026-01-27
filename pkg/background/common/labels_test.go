package common

import (
	"strings"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// mockObject implements the Object interface for testing
type mockObject struct {
	name       string
	namespace  string
	kind       string
	apiVersion string
	uid        types.UID
}

func (m *mockObject) GetName() string       { return m.name }
func (m *mockObject) GetNamespace() string  { return m.namespace }
func (m *mockObject) GetKind() string       { return m.kind }
func (m *mockObject) GetAPIVersion() string { return m.apiVersion }
func (m *mockObject) GetUID() types.UID     { return m.uid }

func TestTrimByLength_ShortStringRemainsUnchanged(t *testing.T) {
	result := trimByLength("short", 63)
	assert.Equal(t, "short", result)
}

func TestTrimByLength_StringAtExactLimitRemainsUnchanged(t *testing.T) {
	input := strings.Repeat("a", 63)
	result := trimByLength(input, 63)
	assert.Equal(t, input, result)
	assert.Len(t, result, 63)
}

func TestTrimByLength_LongStringGetsTruncated(t *testing.T) {
	input := strings.Repeat("a", 100)
	result := trimByLength(input, 63)
	assert.Equal(t, strings.Repeat("a", 63), result)
	assert.Len(t, result, 63)
}

func TestTrimByLength_EmptyStringReturnsEmpty(t *testing.T) {
	result := trimByLength("", 63)
	assert.Equal(t, "", result)
}

func TestMutateLabelsSet_WithNamespacedPolicyAndTrigger(t *testing.T) {
	trigger := &mockObject{
		name:       "my-pod",
		namespace:  "test-ns",
		kind:       "Pod",
		apiVersion: "v1",
		uid:        "abc-123",
	}

	result := MutateLabelsSet("default/my-policy", trigger)

	assert.Equal(t, "my-policy", result[kyvernov2.URMutatePolicyLabel])
	assert.Equal(t, "my-pod", result[kyvernov2.URMutateTriggerNameLabel])
	assert.Equal(t, "test-ns", result[kyvernov2.URMutateTriggerNSLabel])
	assert.Equal(t, "Pod", result[kyvernov2.URMutateTriggerKindLabel])
	assert.Equal(t, "v1", result[kyvernov2.URMutateTriggerAPIVersionLabel])
}

func TestMutateLabelsSet_WithClusterPolicy(t *testing.T) {
	trigger := &mockObject{
		name:       "my-deployment",
		namespace:  "production",
		kind:       "Deployment",
		apiVersion: "apps/v1",
		uid:        "def-456",
	}

	result := MutateLabelsSet("cluster-policy", trigger)

	assert.Equal(t, "cluster-policy", result[kyvernov2.URMutatePolicyLabel])
	assert.Equal(t, "my-deployment", result[kyvernov2.URMutateTriggerNameLabel])
	assert.Equal(t, "production", result[kyvernov2.URMutateTriggerNSLabel])
	assert.Equal(t, "Deployment", result[kyvernov2.URMutateTriggerKindLabel])
	// API version slash gets replaced with dash
	assert.Equal(t, "apps-v1", result[kyvernov2.URMutateTriggerAPIVersionLabel])
}

func TestMutateLabelsSet_WithNilTrigger(t *testing.T) {
	result := MutateLabelsSet("default/my-policy", nil)

	assert.Equal(t, "my-policy", result[kyvernov2.URMutatePolicyLabel])
	assert.Len(t, result, 1, "should only have policy label when trigger is nil")
}

func TestMutateLabelsSet_TruncatesLongTriggerName(t *testing.T) {
	longName := strings.Repeat("a", 100)
	trigger := &mockObject{
		name:       longName,
		namespace:  "default",
		kind:       "ConfigMap",
		apiVersion: "v1",
	}

	result := MutateLabelsSet("my-policy", trigger)

	assert.Len(t, result[kyvernov2.URMutateTriggerNameLabel], 63, "trigger name should be truncated to 63 chars")
}

func TestMutateLabelsSet_SkipsEmptyAPIVersion(t *testing.T) {
	trigger := &mockObject{
		name:       "test-resource",
		namespace:  "default",
		kind:       "CustomResource",
		apiVersion: "",
	}

	result := MutateLabelsSet("my-policy", trigger)

	_, exists := result[kyvernov2.URMutateTriggerAPIVersionLabel]
	assert.False(t, exists, "should not include apiVersion label when empty")
}

func TestGenerateLabelsSet_ExtractsPolicyNameFromNamespacedKey(t *testing.T) {
	result := GenerateLabelsSet("default/generate-policy")

	assert.Equal(t, "generate-policy", result[kyvernov2.URGeneratePolicyLabel])
	assert.Len(t, result, 1)
}

func TestGenerateLabelsSet_UsesFullNameForClusterPolicy(t *testing.T) {
	result := GenerateLabelsSet("cluster-generate-policy")

	assert.Equal(t, "cluster-generate-policy", result[kyvernov2.URGeneratePolicyLabel])
}

func TestGenerateLabelsSet_HandlesComplexNamespace(t *testing.T) {
	result := GenerateLabelsSet("kube-system/system-policy")

	assert.Equal(t, "system-policy", result[kyvernov2.URGeneratePolicyLabel])
}

func TestPolicyInfo_SetsLabelsForClusterPolicy(t *testing.T) {
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "require-labels",
		},
	}
	labels := make(map[string]string)

	PolicyInfo(labels, policy, "check-team-label")

	assert.Equal(t, "require-labels", labels[GeneratePolicyLabel])
	assert.Equal(t, "", labels[GeneratePolicyNamespaceLabel])
	assert.Equal(t, "check-team-label", labels[GenerateRuleLabel])
}

func TestPolicyInfo_SetsLabelsForNamespacedPolicy(t *testing.T) {
	policy := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns-policy",
			Namespace: "production",
		},
	}
	labels := make(map[string]string)

	PolicyInfo(labels, policy, "validate-annotations")

	assert.Equal(t, "ns-policy", labels[GeneratePolicyLabel])
	assert.Equal(t, "production", labels[GeneratePolicyNamespaceLabel])
	assert.Equal(t, "validate-annotations", labels[GenerateRuleLabel])
}

func TestTriggerInfo_SetsPodTriggerLabels(t *testing.T) {
	trigger := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
				"uid":       "pod-uid-123",
			},
		},
	}
	labels := make(map[string]string)

	TriggerInfo(labels, trigger)

	assert.Equal(t, "v1", labels[GenerateTriggerVersionLabel])
	assert.Equal(t, "", labels[GenerateTriggerGroupLabel])
	assert.Equal(t, "Pod", labels[GenerateTriggerKindLabel])
	assert.Equal(t, "default", labels[GenerateTriggerNSLabel])
	assert.Equal(t, "pod-uid-123", labels[GenerateTriggerUIDLabel])
}

func TestTriggerInfo_SetsDeploymentTriggerWithAPIGroup(t *testing.T) {
	trigger := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "my-app",
				"namespace": "production",
				"uid":       "deploy-uid-456",
			},
		},
	}
	labels := make(map[string]string)

	TriggerInfo(labels, trigger)

	assert.Equal(t, "v1", labels[GenerateTriggerVersionLabel])
	assert.Equal(t, "apps", labels[GenerateTriggerGroupLabel])
	assert.Equal(t, "Deployment", labels[GenerateTriggerKindLabel])
	assert.Equal(t, "production", labels[GenerateTriggerNSLabel])
}

func TestTriggerInfo_HandlesClusterScopedResource(t *testing.T) {
	trigger := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": "new-namespace",
				"uid":  "ns-uid-789",
			},
		},
	}
	labels := make(map[string]string)

	TriggerInfo(labels, trigger)

	assert.Equal(t, "Namespace", labels[GenerateTriggerKindLabel])
	assert.Equal(t, "", labels[GenerateTriggerNSLabel], "namespace should be empty for cluster-scoped resources")
}

func TestTagSource_AddsCloneSourceLabel(t *testing.T) {
	labels := make(map[string]string)
	obj := &mockObject{
		name:      "source-configmap",
		namespace: "default",
		kind:      "ConfigMap",
	}

	TagSource(labels, obj)

	assert.Contains(t, labels, GenerateTypeCloneSourceLabel)
}

func TestManageLabels_AddsAllRequiredLabelsToNewResource(t *testing.T) {
	unstr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "generated-secret",
				"namespace": "default",
			},
		},
	}
	trigger := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "trigger-cm",
				"namespace": "default",
				"uid":       "trigger-uid",
			},
		},
	}
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "generate-secrets",
		},
	}

	ManageLabels(unstr, trigger, policy, "copy-secret")

	resultLabels := unstr.GetLabels()
	assert.NotNil(t, resultLabels)
	assert.Equal(t, kyverno.ValueKyvernoApp, resultLabels[kyverno.LabelAppManagedBy])
	assert.Equal(t, "generate-secrets", resultLabels[GeneratePolicyLabel])
	assert.Equal(t, "copy-secret", resultLabels[GenerateRuleLabel])
	assert.Equal(t, "ConfigMap", resultLabels[GenerateTriggerKindLabel])
}

func TestManageLabels_PreservesExistingLabels(t *testing.T) {
	unstr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "generated-secret",
				"namespace": "default",
			},
		},
	}
	unstr.SetLabels(map[string]string{
		"existing-label": "should-remain",
		"app":            "my-app",
	})
	trigger := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "trigger-pod",
				"namespace": "test",
				"uid":       "pod-uid",
			},
		},
	}
	policy := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns-policy",
			Namespace: "test",
		},
	}

	ManageLabels(unstr, trigger, policy, "generate-networkpolicy")

	resultLabels := unstr.GetLabels()
	assert.Equal(t, "should-remain", resultLabels["existing-label"], "existing labels should be preserved")
	assert.Equal(t, "my-app", resultLabels["app"], "existing labels should be preserved")
	assert.Equal(t, kyverno.ValueKyvernoApp, resultLabels[kyverno.LabelAppManagedBy])
}
