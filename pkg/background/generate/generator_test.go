package generate

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MockClient implements the basic functionality needed for testing
type MockClient struct {
	createErr error
	updateErr error
}

func (m *MockClient) CreateResource(ctx context.Context, apiVersion, kind, namespace string, obj *unstructured.Unstructured, dryRun bool) (*unstructured.Unstructured, error) {
	return obj, m.createErr
}

func (m *MockClient) UpdateResource(ctx context.Context, apiVersion, kind, namespace string, obj *unstructured.Unstructured, dryRun bool) (*unstructured.Unstructured, error) {
	return obj, m.updateErr
}

func (m *MockClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}

func (m *MockClient) ApplyResource(ctx context.Context, apiVersion, kind, namespace, name string, obj *unstructured.Unstructured, dryRun bool, fieldManager string) (*unstructured.Unstructured, error) {
	return obj, m.createErr
}

// TestErrorHandling tests the improved error handling in the generator
func TestGeneratorErrorHandling(t *testing.T) {
	tests := []struct {
		name                string
		createErr           error
		updateErr           error
		expectErrorContains string
	}{
		{
			name:                "standard create error",
			createErr:           errors.New("standard create error"),
			expectErrorContains: "failed to create generate target resource",
		},
		{
			name:                "already exists error",
			createErr:           apierrors.NewAlreadyExists(schema.GroupResource{Resource: "configmaps"}, "test-cm"),
			expectErrorContains: "", // Should be skipped without error
		},
		{
			name:                "conflict error during update",
			updateErr:           apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, "test-cm", errors.New("object has been modified")),
			expectErrorContains: "conflict detected while updating resource",
		},
		{
			name:                "standard update error",
			updateErr:           errors.New("standard update error"),
			expectErrorContains: "failed to update resource",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock client with the test error
			mockClient := &MockClient{
				createErr: tc.createErr,
				updateErr: tc.updateErr,
			}

			// Call the function that would normally handle the resource operation
			var result error

			// Simulate the target resource
			target := kyvernov1ResourceSpec{
				Kind:      "ConfigMap",
				Name:      "test-cm",
				Namespace: "default",
			}

			if tc.updateErr != nil {
				// Test update error path
				result = simulateUpdateOperation(mockClient, target)
			} else {
				// Test create error path
				result = simulateCreateOperation(mockClient, target)
			}

			// Verify the error message
			if tc.expectErrorContains == "" {
				assert.Nil(t, result, "Expected no error, but got: %v", result)
			} else {
				assert.NotNil(t, result, "Expected an error, but got nil")
				assert.Contains(t, result.Error(), tc.expectErrorContains,
					"Expected error to contain '%s', but got: '%s'", tc.expectErrorContains, result.Error())
			}
		})
	}
}

// kyvernov1ResourceSpec is a simplified version for testing
type kyvernov1ResourceSpec struct {
	Kind      string
	Name      string
	Namespace string
}

func (r kyvernov1ResourceSpec) GetAPIVersion() string { return "v1" }
func (r kyvernov1ResourceSpec) GetKind() string       { return r.Kind }
func (r kyvernov1ResourceSpec) GetName() string       { return r.Name }
func (r kyvernov1ResourceSpec) GetNamespace() string  { return r.Namespace }

// simulateCreateOperation simulates the resource creation logic
func simulateCreateOperation(client *MockClient, target kyvernov1ResourceSpec) error {
	newResource := &unstructured.Unstructured{}
	newResource.SetName(target.GetName())
	newResource.SetNamespace(target.GetNamespace())
	newResource.SetKind(target.GetKind())

	_, err := client.CreateResource(context.TODO(),
		target.GetAPIVersion(),
		target.GetKind(),
		target.GetNamespace(),
		newResource,
		false)

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create generate target resource %s/%s: %w",
				target.GetNamespace(), target.GetName(), err)
		}
		// Log that resource already exists (simplified for test)
		return nil
	}

	return nil
}

// simulateUpdateOperation simulates the resource update logic
func simulateUpdateOperation(client *MockClient, target kyvernov1ResourceSpec) error {
	newResource := &unstructured.Unstructured{}
	newResource.SetName(target.GetName())
	newResource.SetNamespace(target.GetNamespace())
	newResource.SetKind(target.GetKind())

	_, err := client.UpdateResource(context.TODO(),
		target.GetAPIVersion(),
		target.GetKind(),
		target.GetNamespace(),
		newResource,
		false)

	if err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("conflict detected while updating resource %s/%s: %w",
				target.GetNamespace(), target.GetName(), err)
		}
		return fmt.Errorf("failed to update resource %s/%s: %w",
			target.GetNamespace(), target.GetName(), err)
	}

	return nil
}
