package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

type mockRoleBindingLister struct {
	roleBindings []*rbacv1.RoleBinding
	err          error
}

func (m *mockRoleBindingLister) List(selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.roleBindings, nil
}

func (m *mockRoleBindingLister) RoleBindings(namespace string) rbacv1listers.RoleBindingNamespaceLister {
	return nil
}

type mockClusterRoleBindingLister struct {
	clusterRoleBindings []*rbacv1.ClusterRoleBinding
	err                 error
}

func (m *mockClusterRoleBindingLister) List(selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.clusterRoleBindings, nil
}

func (m *mockClusterRoleBindingLister) Get(name string) (*rbacv1.ClusterRoleBinding, error) {
	return nil, nil
}

func newEnrichTestAdmissionRequest() AdmissionRequest {
	return AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Operation: admissionv1.Create,
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
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
		},
	}
}

func TestWithRoles(t *testing.T) {
	tests := []struct {
		name             string
		rbLister         *mockRoleBindingLister
		crbLister        *mockClusterRoleBindingLister
		request          AdmissionRequest
		wantRoles        []string
		wantClusterRoles []string
		wantInnerCalled  bool
		wantError        bool
	}{
		{
			name: "successfully enriches with roles and cluster roles",
			rbLister: &mockRoleBindingLister{
				roleBindings: []*rbacv1.RoleBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-rb",
							Namespace: "default",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind: "User",
								Name: "test-user",
							},
						},
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "test-role",
						},
					},
				},
			},
			crbLister: &mockClusterRoleBindingLister{
				clusterRoleBindings: []*rbacv1.ClusterRoleBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-crb",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind: "User",
								Name: "test-user",
							},
						},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "test-cluster-role",
						},
					},
				},
			},
			request: func() AdmissionRequest {
				req := newEnrichTestAdmissionRequest()
				req.UserInfo = authenticationv1.UserInfo{Username: "test-user"}
				return req
			}(),
			wantRoles:        []string{"default:test-role"},
			wantClusterRoles: []string{"test-cluster-role"},
			wantInnerCalled:  true,
			wantError:        false,
		},
		{
			name: "handles lister error",
			rbLister: &mockRoleBindingLister{
				err: errors.New("failed to list rolebindings"),
			},
			crbLister: &mockClusterRoleBindingLister{},
			request: func() AdmissionRequest {
				req := newEnrichTestAdmissionRequest()
				req.UserInfo = authenticationv1.UserInfo{Username: "test-user"}
				return req
			}(),
			wantInnerCalled: false,
			wantError:       true,
		},
		{
			name:      "handles empty role bindings",
			rbLister:  &mockRoleBindingLister{roleBindings: []*rbacv1.RoleBinding{}},
			crbLister: &mockClusterRoleBindingLister{clusterRoleBindings: []*rbacv1.ClusterRoleBinding{}},
			request: func() AdmissionRequest {
				req := newEnrichTestAdmissionRequest()
				req.UserInfo = authenticationv1.UserInfo{Username: "test-user"}
				return req
			}(),
			wantRoles:        nil,
			wantClusterRoles: nil,
			wantInnerCalled:  true,
			wantError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerCalled := false
			var capturedRequest AdmissionRequest

			inner := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				innerCalled = true
				capturedRequest = request
				return AdmissionResponse{Allowed: true}
			}

			handler := AdmissionHandler(inner).WithRoles(tt.rbLister, tt.crbLister)
			response := handler(context.TODO(), logr.Discard(), tt.request, time.Now())

			assert.Equal(t, tt.wantInnerCalled, innerCalled, "inner handler called status mismatch")

			if tt.wantError {
				assert.False(t, response.Allowed, "response should not be allowed on error")
			} else if tt.wantInnerCalled {
				assert.Equal(t, tt.wantRoles, capturedRequest.Roles, "roles mismatch")
				assert.Equal(t, tt.wantClusterRoles, capturedRequest.ClusterRoles, "cluster roles mismatch")
				assert.True(t, response.Allowed, "response should be allowed on success")
			}
		})
	}
}

func TestWithTopLevelGVK(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func() dclient.IDiscovery
		request         AdmissionRequest
		wantGVK         schema.GroupVersionKind
		wantInnerCalled bool
	}{
		{
			name: "calls inner handler with discovery client",
			setupClient: func() dclient.IDiscovery {
				return dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
					{Group: "", Version: "v1", Resource: "pods"},
				})
			},
			request: newEnrichTestAdmissionRequest(),
			// Note: fake client's GetGVKFromGVR returns empty GVK
			wantGVK: schema.GroupVersionKind{
				Group:   "",
				Version: "",
				Kind:    "",
			},
			wantInnerCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerCalled := false
			var capturedRequest AdmissionRequest

			inner := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				innerCalled = true
				capturedRequest = request
				return AdmissionResponse{Allowed: true}
			}

			client := tt.setupClient()
			handler := AdmissionHandler(inner).WithTopLevelGVK(client)
			response := handler(context.TODO(), logr.Discard(), tt.request, time.Now())

			assert.Equal(t, tt.wantInnerCalled, innerCalled, "inner handler called status mismatch")
			assert.Equal(t, tt.wantGVK, capturedRequest.GroupVersionKind, "GVK mismatch")
			assert.True(t, response.Allowed, "response should be allowed on success")
		})
	}
}

func TestWithRolesAndGVKChained(t *testing.T) {
	rbLister := &mockRoleBindingLister{
		roleBindings: []*rbacv1.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind: "User",
						Name: "test-user",
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "Role",
					Name: "test-role",
				},
			},
		},
	}

	crbLister := &mockClusterRoleBindingLister{
		clusterRoleBindings: []*rbacv1.ClusterRoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-crb",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind: "User",
						Name: "test-user",
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "test-cluster-role",
				},
			},
		},
	}

	registeredGVRs := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
	}
	client := dclient.NewFakeDiscoveryClient(registeredGVRs)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Operation: admissionv1.Create,
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
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
			UserInfo: authenticationv1.UserInfo{
				Username: "test-user",
			},
		},
	}

	var capturedRequest AdmissionRequest
	inner := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		capturedRequest = request
		return AdmissionResponse{Allowed: true}
	}

	handler := AdmissionHandler(inner).WithRoles(rbLister, crbLister).WithTopLevelGVK(client)
	response := handler(context.TODO(), logr.Discard(), request, time.Now())

	assert.True(t, response.Allowed)
	assert.Equal(t, []string{"default:test-role"}, capturedRequest.Roles)
	assert.Equal(t, []string{"test-cluster-role"}, capturedRequest.ClusterRoles)
	// Note: fake client's GetGVKFromGVR returns empty GVK
	assert.Equal(t, schema.GroupVersionKind{}, capturedRequest.GroupVersionKind)
}
