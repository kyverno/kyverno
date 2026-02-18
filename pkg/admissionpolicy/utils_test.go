package admissionpolicy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/auth/checker"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	mutatingPoliciesBeta  = "admissionregistration.k8s.io/v1beta1/mutatingadmissionpolicies"
	mutatingPoliciesAlpha = "admissionregistration.k8s.io/v1alpha1/mutatingadmissionpolicies"
	mutatingBindingsBeta  = "admissionregistration.k8s.io/v1beta1/mutatingadmissionpolicybindings"
	mutatingBindingsAlpha = "admissionregistration.k8s.io/v1alpha1/mutatingadmissionpolicybindings"
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
					"admissionregistration.k8s.io/v1/validatingadmissionpolicies": true,
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
					"admissionregistration.k8s.io/v1/validatingadmissionpolicybindings": true,
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
			name: "v1beta1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}},
				},
			},
			expect: true,
		},
		{
			name: "v1beta1 missing, v1alpha1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1alpha1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}},
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

func TestIsValidatingAdmissionPolicyRegistered(t *testing.T) {
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
					APIResources: []metav1.APIResource{{Name: "validatingadmissionpolicies"}},
				},
			},
			expect: true,
		},
		{
			name: "v1 missing, v1beta1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "validatingadmissionpolicies"}},
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
			res, err := IsValidatingAdmissionPolicyRegistered(client)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expect, res)
		})
	}
}

func TestCollectParams(t *testing.T) {
	denyAction := admissionregistrationv1.DenyAction

	tests := []struct {
		name           string
		paramKind      *admissionregistrationv1.ParamKind
		paramRef       *admissionregistrationv1.ParamRef
		namespace      string
		mockClient     *mockEngineClient
		expectedLen    int
		expectedErrMsg string
	}{
		{
			name: "invalid api version",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "invalid/version/extra",
				Kind:       "ConfigMap",
			},
			paramRef:       &admissionregistrationv1.ParamRef{},
			mockClient:     &mockEngineClient{},
			expectedErrMsg: "can't parse the parameter resource group version",
		},
		{
			name: "isNamespaced error",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			paramRef: &admissionregistrationv1.ParamRef{},
			mockClient: &mockEngineClient{
				isNamespacedErr: errors.New("discovery failed"),
			},
			expectedErrMsg: "failed to check if resource is namespaced or not (discovery failed)",
		},
		{
			name: "cluster scoped resource with namespace in paramRef",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "Node",
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Namespace: "default",
			},
			mockClient: &mockEngineClient{
				isNamespacedResp: false,
			},
			expectedErrMsg: "paramRef.namespace must not be provided for a cluster-scoped `paramKind`",
		},
		{
			name: "namespaced resource, no namespace provided anywhere",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			paramRef:  &admissionregistrationv1.ParamRef{},
			namespace: "",
			mockClient: &mockEngineClient{
				isNamespacedResp: true,
			},
			expectedErrMsg: "can't use namespaced paramRef to match cluster-scoped resources",
		},
		{
			name: "get by name success",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Name:      "my-config",
				Namespace: "default",
			},
			mockClient: &mockEngineClient{
				isNamespacedResp: true,
				getResourceResp:  &unstructured.Unstructured{},
			},
			expectedLen: 1,
		},
		{
			name: "get by name error",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Name: "my-config",
			},
			namespace: "default",
			mockClient: &mockEngineClient{
				isNamespacedResp: true,
				getResourceErr:   errors.New("not found"),
			},
			expectedErrMsg: "not found",
		},
		{
			name: "list by selector success",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			},
			namespace: "default",
			mockClient: &mockEngineClient{
				isNamespacedResp: true,
				listResourceResp: &unstructured.UnstructuredList{
					Items: []unstructured.Unstructured{{}, {}},
				},
			},
			expectedLen: 2,
		},
		{
			name: "list by selector error",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Selector: &metav1.LabelSelector{},
			},
			namespace: "default",
			mockClient: &mockEngineClient{
				isNamespacedResp: true,
				listResourceErr:  errors.New("list failed"),
			},
			expectedErrMsg: "list failed",
		},
		{
			name: "not found action deny",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Selector:                &metav1.LabelSelector{},
				ParameterNotFoundAction: &denyAction,
			},
			namespace: "default",
			mockClient: &mockEngineClient{
				isNamespacedResp: true,
				listResourceResp: &unstructured.UnstructuredList{Items: []unstructured.Unstructured{}},
			},
			expectedErrMsg: "no params found",
		},
		{
			name: "not found action allow (default)",
			paramKind: &admissionregistrationv1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Selector: &metav1.LabelSelector{},
			},
			namespace: "default",
			mockClient: &mockEngineClient{
				isNamespacedResp: true,
				listResourceResp: &unstructured.UnstructuredList{Items: []unstructured.Unstructured{}},
			},
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := CollectParams(context.TODO(), tt.mockClient, tt.paramKind, tt.paramRef, tt.namespace)
			if tt.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
				assert.Len(t, res, tt.expectedLen)
			}
		})
	}
}
