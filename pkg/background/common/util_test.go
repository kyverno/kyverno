package common

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/config"
	"gotest.tools/v3/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

func TestResourceSpecFromUnstructured(t *testing.T) {
	tests := []struct {
		name     string
		obj      unstructured.Unstructured
		wantKind string
		wantName string
		wantNS   string
		wantAPI  string
	}{
		{
			name: "pod resource",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "default",
						"uid":       "abc-123",
					},
				},
			},
			wantKind: "Pod",
			wantName: "test-pod",
			wantNS:   "default",
			wantAPI:  "v1",
		},
		{
			name: "deployment resource",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "my-app",
						"namespace": "production",
						"uid":       "def-456",
					},
				},
			},
			wantKind: "Deployment",
			wantName: "my-app",
			wantNS:   "production",
			wantAPI:  "apps/v1",
		},
		{
			name: "cluster scoped resource",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata": map[string]interface{}{
						"name": "my-namespace",
						"uid":  "ns-123",
					},
				},
			},
			wantKind: "Namespace",
			wantName: "my-namespace",
			wantNS:   "",
			wantAPI:  "v1",
		},
		{
			name: "custom resource",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kyverno.io/v1",
					"kind":       "ClusterPolicy",
					"metadata": map[string]interface{}{
						"name": "require-labels",
						"uid":  "pol-789",
					},
				},
			},
			wantKind: "ClusterPolicy",
			wantName: "require-labels",
			wantNS:   "",
			wantAPI:  "kyverno.io/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResourceSpecFromUnstructured(tt.obj)

			if result.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", result.Kind, tt.wantKind)
			}
			if result.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", result.Name, tt.wantName)
			}
			if result.Namespace != tt.wantNS {
				t.Errorf("Namespace = %q, want %q", result.Namespace, tt.wantNS)
			}
			if result.APIVersion != tt.wantAPI {
				t.Errorf("APIVersion = %q, want %q", result.APIVersion, tt.wantAPI)
			}
		})
	}
}

func TestResourceSpecFromUnstructured_HasUID(t *testing.T) {
	obj := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
				"uid":       "uid-123-456",
			},
		},
	}

	result := ResourceSpecFromUnstructured(obj)

	if result.UID != "uid-123-456" {
		t.Errorf("UID = %q, want %q", result.UID, "uid-123-456")
	}
}

func TestResourceSpecFromUnstructured_ReturnsResourceSpec(t *testing.T) {
	obj := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "my-secret",
				"namespace": "kyverno",
			},
		},
	}

	result := ResourceSpecFromUnstructured(obj)

	// Verify the result is of type ResourceSpec
	var _ kyverno.ResourceSpec = result
}

func TestResourceSpecFromUnstructured_EmptyObject(t *testing.T) {
	obj := unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	result := ResourceSpecFromUnstructured(obj)

	if result.Kind != "" {
		t.Errorf("Kind should be empty for empty object, got %q", result.Kind)
	}
	if result.Name != "" {
		t.Errorf("Name should be empty for empty object, got %q", result.Name)
	}
}

func TestUpdateStatus_DeleteOnExceedRetryLimit(t *testing.T) {
	client := fake.NewSimpleClientset()
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: config.KyvernoNamespace(),
		},
		Status: kyvernov2.UpdateRequestStatus{
			RetryCount: 4, // exceeds retry limit of 3
		},
	}
	_, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
	assert.NilError(t, err)

	res, err := UpdateStatus(context.TODO(), client, nil, "test-ur", kyvernov2.Failed, "failure message", nil)
	// When retry limit is exceeded, retryOrDeleteOnFailure deletes the UR from the cluster.
	// UpdateStatus must return (nil, nil) and NOT fail with a 404 NotFound error!
	assert.NilError(t, err)
	assert.Assert(t, res == nil)

	// Verify that the UR was deleted from the cluster
	_, err = client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "test-ur", metav1.GetOptions{})
	assert.Assert(t, err != nil, "expected UR to be deleted from cluster")
}

func TestUpdateStatus_RetryUnderLimit(t *testing.T) {
	client := fake.NewSimpleClientset()
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: config.KyvernoNamespace(),
		},
		Status: kyvernov2.UpdateRequestStatus{
			RetryCount: 1, // under retry limit of 3
		},
	}
	_, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
	assert.NilError(t, err)

	res, err := UpdateStatus(context.TODO(), client, nil, "test-ur", kyvernov2.Failed, "failure message", nil)
	assert.NilError(t, err)
	assert.Assert(t, res != nil)

	// Verify the UR was updated with incremented retry count and Failed state
	fetched, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "test-ur", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, fetched.Status.RetryCount, 2)
	assert.Equal(t, fetched.Status.State, kyvernov2.Failed)
	assert.Equal(t, fetched.Status.Message, "failure message")
}

func TestUpdateStatus_CompletedState(t *testing.T) {
	client := fake.NewSimpleClientset()
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: config.KyvernoNamespace(),
		},
		Status: kyvernov2.UpdateRequestStatus{
			RetryCount: 1,
		},
	}
	_, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
	assert.NilError(t, err)

	res, err := UpdateStatus(context.TODO(), client, nil, "test-ur", kyvernov2.Completed, "", nil)
	assert.NilError(t, err)
	assert.Assert(t, res != nil)

	// Verify the UR was updated to Completed without incrementing retry count
	fetched, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "test-ur", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, fetched.Status.RetryCount, 1)
	assert.Equal(t, fetched.Status.State, kyvernov2.Completed)
}

func TestUpdateStatus_DeleteConcurrentNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: config.KyvernoNamespace(),
		},
		Status: kyvernov2.UpdateRequestStatus{
			RetryCount: 4, // exceeds retry limit of 3
		},
	}
	_, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
	assert.NilError(t, err)

	// Simulate concurrent deletion where Delete returns NotFound
	client.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: kyvernov2.SchemeGroupVersion.Group, Resource: "updaterequests"}, "test-ur")
	})

	updateCalled := false
	client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" {
			updateCalled = true
		}
		return false, nil, nil
	})

	res, err := UpdateStatus(context.TODO(), client, nil, "test-ur", kyvernov2.Failed, "failure message", nil)
	assert.NilError(t, err)
	assert.Assert(t, res == nil)
	assert.Equal(t, updateCalled, false, "no status update should be issued after deletion")
}

func TestUpdateStatus_DeleteError(t *testing.T) {
	client := fake.NewSimpleClientset()
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: config.KyvernoNamespace(),
		},
		Status: kyvernov2.UpdateRequestStatus{
			RetryCount: 4,
		},
	}
	_, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
	assert.NilError(t, err)

	client.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("transient delete error")
	})

	_, err = UpdateStatus(context.TODO(), client, nil, "test-ur", kyvernov2.Failed, "failure message", nil)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "exceeds retry limit, failed to delete the UR"))
}

func TestUpdateStatus_RetryLimitBoundary(t *testing.T) {
	client := fake.NewSimpleClientset()
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: config.KyvernoNamespace(),
		},
		Status: kyvernov2.UpdateRequestStatus{
			RetryCount: 3, // exactly equal to limit of 3 (should not delete, 3 > 3 is false)
		},
	}
	_, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
	assert.NilError(t, err)

	deleteCalled := false
	client.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deleteCalled = true
		return false, nil, nil
	})

	res, err := UpdateStatus(context.TODO(), client, nil, "test-ur", kyvernov2.Failed, "failure message", nil)
	assert.NilError(t, err)
	assert.Assert(t, res != nil)
	assert.Equal(t, deleteCalled, false, "UR should not be deleted when RetryCount equals limit")

	fetched, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "test-ur", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, fetched.Status.RetryCount, 4)
	assert.Equal(t, fetched.Status.State, kyvernov2.Failed)
}
