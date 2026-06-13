package admissionpolicy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/auth/checker"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	mutatingPoliciesV1    = "admissionregistration.k8s.io/v1/mutatingadmissionpolicies"
	mutatingPoliciesBeta  = "admissionregistration.k8s.io/v1beta1/mutatingadmissionpolicies"
	mutatingPoliciesAlpha = "admissionregistration.k8s.io/v1alpha1/mutatingadmissionpolicies"

	mutatingBindingsV1    = "admissionregistration.k8s.io/v1/mutatingadmissionpolicybindings"
	mutatingBindingsBeta  = "admissionregistration.k8s.io/v1beta1/mutatingadmissionpolicybindings"
	mutatingBindingsAlpha = "admissionregistration.k8s.io/v1alpha1/mutatingadmissionpolicybindings"

	validatingPoliciesV1 = "admissionregistration.k8s.io/v1/validatingadmissionpolicies"
	validatingBindingsV1 = "admissionregistration.k8s.io/v1/validatingadmissionpolicybindings"
)

type mockAuthChecker struct {
	results map[string]bool
	err     error
}

func (m *mockAuthChecker) Check(ctx context.Context, group, version, resource, subresource, name, namespace, verb string) (*checker.AuthResult, error) {
	if m.err != nil {
		return nil, m.err
	}

	key := fmt.Sprintf("%s/%s/%s", group, version, resource)

	if allowed, ok := m.results[key]; ok {
		return &checker.AuthResult{Allowed: allowed}, nil
	}

	return &checker.AuthResult{Allowed: false}, nil
}

type mockEngineClient struct {
	engineapi.Client

	isNamespacedResp bool
	isNamespacedErr  error

	getResourceResp *unstructured.Unstructured
	getResourceErr  error

	listResourceResp *unstructured.UnstructuredList
	listResourceErr  error
}

func (m *mockEngineClient) IsNamespaced(group, version, kind string) (bool, error) {
	return m.isNamespacedResp, m.isNamespacedErr
}

func (m *mockEngineClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return m.getResourceResp, m.getResourceErr
}

func (m *mockEngineClient) ListResource(ctx context.Context, apiVersion, kind, namespace string, selector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return m.listResourceResp, m.listResourceErr
}

func TestHasValidatingAdmissionPolicyPermission(t *testing.T) {
	tests := []struct {
		name     string
		auth     *mockAuthChecker
		expected bool
	}{
		{
			name: "allowed",
			auth: &mockAuthChecker{
				results: map[string]bool{
					validatingPoliciesV1: true,
				},
			},
			expected: true,
		},
		{
			name:     "denied",
			auth:     &mockAuthChecker{results: map[string]bool{}},
			expected: false,
		},
		{
			name:     "error",
			auth:     &mockAuthChecker{err: errors.New("auth error")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasValidatingAdmissionPolicyPermission(tt.auth))
		})
	}
}

func TestHasValidatingAdmissionPolicyBindingPermission(t *testing.T) {
	tests := []struct {
		name     string
		auth     *mockAuthChecker
		expected bool
	}{
		{
			name: "allowed",
			auth: &mockAuthChecker{
				results: map[string]bool{
					validatingBindingsV1: true,
				},
			},
			expected: true,
		},
		{
			name:     "denied",
			auth:     &mockAuthChecker{results: map[string]bool{}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasValidatingAdmissionPolicyBindingPermission(tt.auth))
		})
	}
}

func TestHasMutatingAdmissionPolicyPermission(t *testing.T) {
	tests := []struct {
		name     string
		auth     *mockAuthChecker
		expected bool
	}{
		{
			name: "v1 allowed",
			auth: &mockAuthChecker{
				results: map[string]bool{
					mutatingPoliciesV1: true,
				},
			},
			expected: true,
		},
		{
			name: "v1beta1 allowed",
			auth: &mockAuthChecker{
				results: map[string]bool{
					mutatingPoliciesBeta: true,
				},
			},
			expected: true,
		},
		{
			name: "v1beta1 denied, v1alpha1 allowed",
			auth: &mockAuthChecker{
				results: map[string]bool{
					mutatingPoliciesBeta:  false,
					mutatingPoliciesAlpha: true,
				},
			},
			expected: true,
		},
		{
			name: "both denied",
			auth: &mockAuthChecker{
				results: map[string]bool{
					mutatingPoliciesBeta:  false,
					mutatingPoliciesAlpha: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasMutatingAdmissionPolicyPermission(tt.auth))
		})
	}
}

func TestHasMutatingAdmissionPolicyBindingPermission(t *testing.T) {
	tests := []struct {
		name     string
		auth     *mockAuthChecker
		expected bool
	}{
		{
			name: "v1 allowed",
			auth: &mockAuthChecker{
				results: map[string]bool{
					mutatingBindingsV1: true,
				},
			},
			expected: true,
		},
		{
			name: "v1beta1 allowed",
			auth: &mockAuthChecker{
				results: map[string]bool{
					mutatingBindingsBeta: true,
				},
			},
			expected: true,
		},
		{
			name: "v1beta1 denied, v1alpha1 allowed",
			auth: &mockAuthChecker{
				results: map[string]bool{
					mutatingBindingsBeta:  false,
					mutatingBindingsAlpha: true,
				},
			},
			expected: true,
		},
		{
			name: "both denied",
			auth: &mockAuthChecker{
				results: map[string]bool{
					mutatingBindingsBeta:  false,
					mutatingBindingsAlpha: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasMutatingAdmissionPolicyBindingPermission(tt.auth))
		})
	}
}

func TestIsMutatingAdmissionPolicyRegistered(t *testing.T) {
	tests := []struct {
		name      string
		resources []*metav1.APIResourceList
		expect    bool
		expectErr bool
	}{
		{
			name: "v1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
			},
			expect: true,
		},
		{
			name: "v1beta1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
			},
			expect: true,
		},
		{
			name: "v1beta1 missing, v1alpha1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1alpha1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
			},
			expect: true,
		},
		{
			name:      "neither present",
			resources: []*metav1.APIResourceList{},
			expect:    false,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset()
			client.Fake.Resources = tt.resources

			res, err := IsMutatingAdmissionPolicyRegistered(client)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expect, res)
		})
	}
}

func TestPreferredMutatingAdmissionPolicyVersion(t *testing.T) {
	tests := []struct {
		name      string
		resources []*metav1.APIResourceList
		expect    MutatingAdmissionPolicyVersion
		expectErr bool
	}{
		{
			name: "v1 preferred when present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1,
		},
		{
			name: "v1 preferred over v1beta1",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1,
		},
		{
			name: "v1beta1 preferred when both resources are present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1beta1,
		},
		{
			name: "v1alpha1 fallback when beta resources are absent",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1alpha1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1alpha1,
		},
		{
			name: "missing binding resource is treated as unsupported",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "fallback to v1alpha1 when v1beta1 is partial",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
					},
				},
				{
					GroupVersion: "admissionregistration.k8s.io/v1alpha1",
					APIResources: []metav1.APIResource{
						{Name: "mutatingadmissionpolicies"},
						{Name: "mutatingadmissionpolicybindings"},
					},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1alpha1,
		},
		{
			name:      "no supported version",
			resources: []*metav1.APIResourceList{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset()
			client.Fake.Resources = tt.resources

			version, err := PreferredMutatingAdmissionPolicyVersion(client)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expect, version)
		})
	}
}

func TestPreferredMutatingAdmissionPolicyVersion_FailFastOnDiscoveryError(t *testing.T) {
	client := fake.NewClientset()

	errForbidden := apierrors.NewForbidden(
		schema.GroupResource{
			Group:    "admissionregistration.k8s.io",
			Resource: "mutatingadmissionpolicies",
		},
		"",
		errors.New("forbidden"),
	)

	client.Fake.PrependReactor("get", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errForbidden
	})

	version, err := PreferredMutatingAdmissionPolicyVersion(client)

	assert.Empty(t, version)
	assert.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err))
}
