package test

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/utils/report"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestResourceVersionTracking(t *testing.T) {
	// Create a mock unstructured resource
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":            "test-configmap",
				"namespace":       "default",
				"resourceVersion": "1234",
			},
			"data": map[string]interface{}{
				"key1": "value1",
			},
		},
	}

	// Create a mock report
	mockReport := &MockReport{
		labels:      make(map[string]string),
		annotations: make(map[string]string),
	}

	// Set resource version labels
	if resource != nil {
		mockReport.labels[report.LabelResourceHash] = report.CalculateResourceHash(*resource)
		mockReport.labels[report.LabelResourceObjectVersion] = resource.GetResourceVersion()
	}

	// Verify the resource hash is set
	if mockReport.labels[report.LabelResourceHash] == "" {
		t.Errorf("Resource hash was not set")
	}

	// Verify the resource version is set
	if mockReport.labels[report.LabelResourceObjectVersion] != "1234" {
		t.Errorf("Resource version was not set correctly, got %s, expected 1234",
			mockReport.labels[report.LabelResourceObjectVersion])
	}

	// Update the resource but keep the same hash (just change resourceVersion)
	updatedResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":            "test-configmap",
				"namespace":       "default",
				"resourceVersion": "5678",
			},
			"data": map[string]interface{}{
				"key1": "value1",
			},
		},
	}

	// First verify that the hash remains the same
	originalHash := mockReport.labels[report.LabelResourceHash]
	newHash := report.CalculateResourceHash(*updatedResource)

	if originalHash != newHash {
		t.Errorf("Resource hash changed unexpectedly, old %s, new %s", originalHash, newHash)
	}

	// Update the report with the new resource
	if updatedResource != nil {
		mockReport.labels[report.LabelResourceHash] = report.CalculateResourceHash(*updatedResource)
		mockReport.labels[report.LabelResourceObjectVersion] = updatedResource.GetResourceVersion()
	}

	// Verify the resource version was updated
	if mockReport.labels[report.LabelResourceObjectVersion] != "5678" {
		t.Errorf("Resource version was not updated correctly, got %s, expected 5678",
			mockReport.labels[report.LabelResourceObjectVersion])
	}
}

// Mock implementation of metav1.Object
type MockReport struct {
	labels      map[string]string
	annotations map[string]string
}

func (m *MockReport) GetName() string {
	return ""
}

func (m *MockReport) SetName(name string) {
}

func (m *MockReport) GetNamespace() string {
	return ""
}

func (m *MockReport) SetNamespace(namespace string) {
}

func (m *MockReport) GetLabels() map[string]string {
	return m.labels
}

func (m *MockReport) SetLabels(labels map[string]string) {
	m.labels = labels
}

func (m *MockReport) GetAnnotations() map[string]string {
	return m.annotations
}

func (m *MockReport) SetAnnotations(annotations map[string]string) {
	m.annotations = annotations
}

func (m *MockReport) GetResourceVersion() string {
	return ""
}

func (m *MockReport) SetResourceVersion(version string) {
}

// Implement other required methods from metav1.Object
func (m *MockReport) GetGenerateName() string {
	return ""
}

func (m *MockReport) SetGenerateName(name string) {
}

func (m *MockReport) GetUID() types.UID {
	return ""
}

func (m *MockReport) SetUID(uid types.UID) {
}

func (m *MockReport) GetGeneration() int64 {
	return 0
}

func (m *MockReport) SetGeneration(generation int64) {
}

func (m *MockReport) GetSelfLink() string {
	return ""
}

func (m *MockReport) SetSelfLink(selfLink string) {
}

func (m *MockReport) GetCreationTimestamp() metav1.Time {
	return metav1.Time{}
}

func (m *MockReport) SetCreationTimestamp(timestamp metav1.Time) {
}

func (m *MockReport) GetDeletionTimestamp() *metav1.Time {
	return nil
}

func (m *MockReport) SetDeletionTimestamp(timestamp *metav1.Time) {
}

func (m *MockReport) GetDeletionGracePeriodSeconds() *int64 {
	return nil
}

func (m *MockReport) SetDeletionGracePeriodSeconds(seconds *int64) {
}

func (m *MockReport) GetFinalizers() []string {
	return nil
}

func (m *MockReport) SetFinalizers(finalizers []string) {
}

func (m *MockReport) GetOwnerReferences() []metav1.OwnerReference {
	return nil
}

func (m *MockReport) SetOwnerReferences(references []metav1.OwnerReference) {
}

func (m *MockReport) GetManagedFields() []metav1.ManagedFieldsEntry {
	return nil
}

func (m *MockReport) SetManagedFields(managedFields []metav1.ManagedFieldsEntry) {
}

// Mock constants
var (
	_ metav1.Object = &MockReport{}
)

// Constants used in test
const (
	LabelResourceHash          = report.LabelResourceHash
	LabelResourceObjectVersion = report.LabelResourceObjectVersion
)
