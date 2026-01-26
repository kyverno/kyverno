package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockConfiguration struct {
	excluded bool
	filtered bool
}

func (m *mockConfiguration) GetDefaultRegistry() string               { return "" }
func (m *mockConfiguration) GetEnableDefaultRegistryMutation() bool   { return false }
func (m *mockConfiguration) GetGenerateSuccessEvents() bool           { return false }
func (m *mockConfiguration) GetWebhook() config.WebhookConfig         { return config.WebhookConfig{} }
func (m *mockConfiguration) GetWebhookAnnotations() map[string]string { return nil }
func (m *mockConfiguration) GetWebhookLabels() map[string]string      { return nil }
func (m *mockConfiguration) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return nil
}
func (m *mockConfiguration) Load(*corev1.ConfigMap) {}
func (m *mockConfiguration) OnChanged(func())       {}
func (m *mockConfiguration) GetUpdateRequestThreshold() int64 {
	return 0
}
func (m *mockConfiguration) GetMaxContextSize() int64 {
	return config.DefaultMaxContextSize
}

func (m *mockConfiguration) IsExcluded(username string, groups []string, roles []string, clusterroles []string) bool {
	return m.excluded
}

func (m *mockConfiguration) ToFilter(kind schema.GroupVersionKind, subresource, namespace, name string) bool {
	return m.filtered
}

func newTestAdmissionRequest(uid string, kind metav1.GroupVersionKind, operation admissionv1.Operation, subResource string) AdmissionRequest {
	return AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Kind:      kind,
			Operation: operation,
			Resource: metav1.GroupVersionResource{
				Group:    kind.Group,
				Version:  kind.Version,
				Resource: "pods",
			},
			SubResource: subResource,
			RequestKind: &metav1.GroupVersionKind{
				Group:   kind.Group,
				Version: kind.Version,
				Kind:    kind.Kind,
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    kind.Group,
				Version:  kind.Version,
				Resource: "pods",
			},
		},
	}
}

func Test_WithFilter(t *testing.T) {
	tests := []struct {
		name            string
		config          config.Configuration
		request         AdmissionRequest
		wantAllowed     bool
		wantInnerCalled bool
	}{{
		name: "excluded by user exclusion",
		config: &mockConfiguration{
			excluded: true,
		},
		request: newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}, admissionv1.Create, ""),
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name: "filtered by resource filter",
		config: &mockConfiguration{
			filtered: true,
		},
		request: newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}, admissionv1.Create, ""),
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:   "filtered kyverno resource - AdmissionReport",
		config: &mockConfiguration{},
		request: newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
			Group:   "kyverno.io",
			Version: "v1alpha2",
			Kind:    "AdmissionReport",
		}, admissionv1.Create, ""),
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:   "filtered kyverno resource - ClusterAdmissionReport",
		config: &mockConfiguration{},
		request: newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
			Group:   "kyverno.io",
			Version: "v1alpha2",
			Kind:    "ClusterAdmissionReport",
		}, admissionv1.Create, ""),
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:   "filtered kyverno resource - BackgroundScanReport",
		config: &mockConfiguration{},
		request: newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
			Group:   "kyverno.io",
			Version: "v1alpha2",
			Kind:    "BackgroundScanReport",
		}, admissionv1.Create, ""),
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:   "filtered kyverno resource - UpdateRequest",
		config: &mockConfiguration{},
		request: newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
			Group:   "kyverno.io",
			Version: "v1beta1",
			Kind:    "UpdateRequest",
		}, admissionv1.Create, ""),
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:   "not filtered - regular Pod",
		config: &mockConfiguration{},
		request: newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}, admissionv1.Create, ""),
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:   "not filtered - ClusterPolicy (not excluded by ExcludeKyvernoResources)",
		config: &mockConfiguration{},
		request: newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
			Group:   "kyverno.io",
			Version: "v1",
			Kind:    "ClusterPolicy",
		}, admissionv1.Create, ""),
		wantAllowed:     false,
		wantInnerCalled: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerCalled := false
			inner := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				innerCalled = true
				return AdmissionResponse{Allowed: false}
			}
			handler := AdmissionHandler(inner).WithFilter(tt.config)
			response := handler(context.TODO(), logr.Discard(), tt.request, time.Now())
			assert.Equal(t, tt.wantAllowed, response.Allowed)
			assert.Equal(t, tt.wantInnerCalled, innerCalled)
		})
	}
}

func Test_WithOperationFilter(t *testing.T) {
	tests := []struct {
		name            string
		operations      []admissionv1.Operation
		requestOp       admissionv1.Operation
		wantAllowed     bool
		wantInnerCalled bool
	}{{
		name:            "allowed operation - CREATE",
		operations:      []admissionv1.Operation{admissionv1.Create},
		requestOp:       admissionv1.Create,
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "allowed operation - UPDATE",
		operations:      []admissionv1.Operation{admissionv1.Update},
		requestOp:       admissionv1.Update,
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "allowed operation - DELETE",
		operations:      []admissionv1.Operation{admissionv1.Delete},
		requestOp:       admissionv1.Delete,
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "allowed multiple operations - CREATE and UPDATE",
		operations:      []admissionv1.Operation{admissionv1.Create, admissionv1.Update},
		requestOp:       admissionv1.Update,
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "filtered operation - UPDATE not in allowed list",
		operations:      []admissionv1.Operation{admissionv1.Create},
		requestOp:       admissionv1.Update,
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:            "filtered operation - DELETE not in allowed list",
		operations:      []admissionv1.Operation{admissionv1.Create, admissionv1.Update},
		requestOp:       admissionv1.Delete,
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:            "empty operations list filters all",
		operations:      []admissionv1.Operation{},
		requestOp:       admissionv1.Create,
		wantAllowed:     true,
		wantInnerCalled: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerCalled := false
			inner := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				innerCalled = true
				return AdmissionResponse{Allowed: false}
			}
			request := newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}, tt.requestOp, "")
			handler := AdmissionHandler(inner).WithOperationFilter(tt.operations...)
			response := handler(context.TODO(), logr.Discard(), request, time.Now())
			assert.Equal(t, tt.wantAllowed, response.Allowed)
			assert.Equal(t, tt.wantInnerCalled, innerCalled)
		})
	}
}

func Test_WithSubResourceFilter(t *testing.T) {
	tests := []struct {
		name            string
		subresources    []string
		requestSubRes   string
		wantAllowed     bool
		wantInnerCalled bool
	}{{
		name:            "allowed subresource - status",
		subresources:    []string{"status"},
		requestSubRes:   "status",
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "allowed subresource - scale",
		subresources:    []string{"scale"},
		requestSubRes:   "scale",
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "allowed multiple subresources",
		subresources:    []string{"status", "scale"},
		requestSubRes:   "scale",
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "empty request subresource is always allowed",
		subresources:    []string{"status"},
		requestSubRes:   "",
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "empty subresource list - empty request allowed",
		subresources:    []string{},
		requestSubRes:   "",
		wantAllowed:     false,
		wantInnerCalled: true,
	}, {
		name:            "filtered subresource - scale not in allowed list",
		subresources:    []string{"status"},
		requestSubRes:   "scale",
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:            "filtered subresource - eviction not in allowed list",
		subresources:    []string{"status", "scale"},
		requestSubRes:   "eviction",
		wantAllowed:     true,
		wantInnerCalled: false,
	}, {
		name:            "empty subresource list filters all non-empty requests",
		subresources:    []string{},
		requestSubRes:   "status",
		wantAllowed:     true,
		wantInnerCalled: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerCalled := false
			inner := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				innerCalled = true
				return AdmissionResponse{Allowed: false}
			}
			request := newTestAdmissionRequest("test-uid", metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}, admissionv1.Create, tt.requestSubRes)
			handler := AdmissionHandler(inner).WithSubResourceFilter(tt.subresources...)
			response := handler(context.TODO(), logr.Discard(), request, time.Now())
			assert.Equal(t, tt.wantAllowed, response.Allowed)
			assert.Equal(t, tt.wantInnerCalled, innerCalled)
		})
	}
}
