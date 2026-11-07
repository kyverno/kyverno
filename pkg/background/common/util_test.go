package common

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/config"
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

func TestUpdateStatus_DeletesURAfterRetryLimit(t *testing.T) {
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-1", Namespace: config.KyvernoNamespace()},
		Status:     kyvernov2.UpdateRequestStatus{RetryCount: 4},
	}
	client := fake.NewSimpleClientset(ur)
	client.PrependReactor("update", "updaterequests", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "updaterequests"}, "ur-1")
	})
	_, err := UpdateStatus(context.TODO(), client, nil, "ur-1", kyvernov2.Failed, "boom", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_, err = client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "ur-1", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected UR to be deleted, got %v", err)
	}
}
