package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestWithProtection_NamespaceControllerDeletion(t *testing.T) {
	called := false
	mockInner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		called = true
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	handler := mockInner.WithProtection(true)

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx",
				},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Delete,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Name:      "test-pod",
			UserInfo: authenticationv1.UserInfo{
				Username: namespaceControllerUsername,
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			OldObject: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	response := handler(context.Background(), logr.Discard(), request, time.Now())

	assert.True(t, called, "Inner handler should be called for namespace controller deletion")
	assert.True(t, response.Allowed, "Namespace controller should be allowed to delete resources")
}

func TestWithProtection_KyvernoManagedResource_NonKyvernoUser(t *testing.T) {
	mockInner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	handler := mockInner.WithProtection(true)

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx",
				},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Update,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Name:      "test-pod",
			UserInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:default:user",
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Object: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	response := handler(context.Background(), logr.Discard(), request, time.Now())

	assert.False(t, response.Allowed, "Non-Kyverno user should not be allowed to modify Kyverno-managed resources")
	assert.NotNil(t, response.Result, "Result should contain error message")
	assert.Contains(t, response.Result.Message, "kyverno managed resource", "Error message should mention Kyverno managed resource")
}

func TestWithProtection_KyvernoManagedResource_KyvernoUser(t *testing.T) {
	called := false
	mockInner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		called = true
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	handler := mockInner.WithProtection(true)

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx",
				},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Update,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Name:      "test-pod",
			UserInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kyverno:kyverno-service-account",
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Object: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	response := handler(context.Background(), logr.Discard(), request, time.Now())

	assert.True(t, called, "Inner handler should be called for Kyverno user")
	assert.True(t, response.Allowed, "Kyverno service account should be allowed to modify Kyverno-managed resources")
}

func TestWithProtection_NonKyvernoManagedResource(t *testing.T) {
	called := false
	mockInner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		called = true
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	handler := mockInner.WithProtection(true)

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "my-app",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx",
				},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Update,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Name:      "test-pod",
			UserInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:default:user",
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Object: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	response := handler(context.Background(), logr.Discard(), request, time.Now())
	assert.True(t, called, "Inner handler should be called for non-Kyverno managed resources")
	assert.True(t, response.Allowed, "Regular user should be allowed to modify non-Kyverno managed resources")
}
