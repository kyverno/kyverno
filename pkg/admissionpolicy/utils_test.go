package admissionpolicy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
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
	validatingPoliciesV1  = "admissionregistration.k8s.io/v1/validatingadmissionpolicies"
	validatingBindingsV1  = "admissionregistration.k8s.io/v1/validatingadmissionpolicybindings"
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
	discoveryCalls   int
	getCalls         int
	listCalls        int

	getResourceResp *unstructured.Unstructured
	getResourceErr  error
	getNamespace    string

	listResourceResp *unstructured.UnstructuredList
	listResourceErr  error
	listNamespace    string
}

func (m *mockEngineClient) IsNamespaced(group, version, kind string) (bool, error) {
	m.discoveryCalls++
	return m.isNamespacedResp, m.isNamespacedErr
}

func (m *mockEngineClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	m.getCalls++
	m.getNamespace = namespace
	return m.getResourceResp, m.getResourceErr
}

func (m *mockEngineClient) ListResource(ctx context.Context, apiVersion, kind, namespace string, selector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	m.listCalls++
	m.listNamespace = namespace
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
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
				},
			},
			expect: true,
		},
		{
			name: "v1beta1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
				},
			},
			expect: true,
		},
		{
			name: "v1beta1 missing, v1alpha1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1alpha1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
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
			name: "v1 preferred when resources are present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
				},
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1,
		},
		{
			name: "v1beta1 preferred when both resources are present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1beta1,
		},
		{
			name: "fallback to v1beta1 when v1 is partial",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}},
				},
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1beta1,
		},
		{
			name: "v1alpha1 fallback when beta resources are absent",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1alpha1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
				},
			},
			expect: MutatingAdmissionPolicyVersionV1alpha1,
		},
		{
			name: "missing binding resource is treated as unsupported",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}},
				},
			},
			expectErr: true,
		},
		{
			name: "fallback to v1alpha1 when v1beta1 is partial",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}},
				},
				{
					GroupVersion: "admissionregistration.k8s.io/v1alpha1",
					APIResources: []metav1.APIResource{{Name: "mutatingadmissionpolicies"}, {Name: "mutatingadmissionpolicybindings"}},
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
	errForbidden := apierrors.NewForbidden(schema.GroupResource{Group: "admissionregistration.k8s.io", Resource: "mutatingadmissionpolicies"}, "", errors.New("forbidden"))
	client.Fake.PrependReactor("get", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errForbidden
	})

	version, err := PreferredMutatingAdmissionPolicyVersion(client)
	assert.Empty(t, version)
	assert.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err))
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
					APIResources: []metav1.APIResource{{Name: "validatingadmissionpolicies"}, {Name: "validatingadmissionpolicybindings"}},
				},
			},
			expect: true,
		},
		{
			name: "v1 missing, v1beta1 present",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1beta1",
					APIResources: []metav1.APIResource{{Name: "validatingadmissionpolicies"}, {Name: "validatingadmissionpolicybindings"}},
				},
			},
			expect: false,
		},
		{
			name: "binding resource is required",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "admissionregistration.k8s.io/v1",
					APIResources: []metav1.APIResource{{Name: "validatingadmissionpolicies"}},
				},
			},
			expect: false,
		},
		{
			name:      "neither present",
			resources: []*metav1.APIResourceList{},
			expect:    false,
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

func TestCollectParamsForPolicy(t *testing.T) {
	t.Parallel()
	configMapKind := &admissionregistrationv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"}
	nodeKind := &admissionregistrationv1.ParamKind{APIVersion: "v1", Kind: "Node"}

	t.Run("defaults to policy namespace", func(t *testing.T) {
		t.Parallel()
		client := &mockEngineClient{
			isNamespacedResp: true,
			getResourceResp:  &unstructured.Unstructured{},
		}
		params, err := CollectParamsForPolicy(
			context.Background(),
			client,
			configMapKind,
			&admissionregistrationv1.ParamRef{Name: "params"},
			"resource-ns",
			&kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: "policy-ns"}},
		)
		assert.NoError(t, err)
		assert.Len(t, params, 1)
		assert.Equal(t, "policy-ns", client.getNamespace)
	})

	t.Run("allows explicit policy namespace", func(t *testing.T) {
		t.Parallel()
		client := &mockEngineClient{
			isNamespacedResp: true,
			getResourceResp:  &unstructured.Unstructured{},
		}
		_, err := CollectParamsForPolicy(
			context.Background(),
			client,
			configMapKind,
			&admissionregistrationv1.ParamRef{Name: "params", Namespace: "policy-ns"},
			"resource-ns",
			&kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: "policy-ns"}},
		)
		assert.NoError(t, err)
		assert.Equal(t, "policy-ns", client.getNamespace)
	})

	t.Run("selector is confined to policy namespace", func(t *testing.T) {
		t.Parallel()
		client := &mockEngineClient{
			isNamespacedResp: true,
			listResourceResp: &unstructured.UnstructuredList{},
		}
		_, err := CollectParamsForPolicy(
			context.Background(),
			client,
			configMapKind,
			&admissionregistrationv1.ParamRef{Selector: &metav1.LabelSelector{}},
			"resource-ns",
			&kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: "policy-ns"}},
		)
		assert.NoError(t, err)
		assert.Equal(t, "policy-ns", client.listNamespace)
	})

	t.Run("rejects foreign namespace before reading", func(t *testing.T) {
		t.Parallel()
		client := &mockEngineClient{isNamespacedResp: true}
		_, err := CollectParamsForPolicy(
			context.Background(),
			client,
			configMapKind,
			&admissionregistrationv1.ParamRef{Name: "params", Namespace: "foreign-ns"},
			"resource-ns",
			&kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: "policy-ns"}},
		)
		assert.ErrorContains(t, err, "must match policy namespace")
		assert.Zero(t, client.getCalls)
		assert.Zero(t, client.listCalls)
	})

	t.Run("rejects cluster scoped kind before reading", func(t *testing.T) {
		t.Parallel()
		client := &mockEngineClient{isNamespacedResp: false}
		_, err := CollectParamsForPolicy(
			context.Background(),
			client,
			nodeKind,
			&admissionregistrationv1.ParamRef{Name: "node"},
			"resource-ns",
			&kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: "policy-ns"}},
		)
		assert.ErrorContains(t, err, "cluster-scoped paramKind is not allowed")
		assert.Zero(t, client.getCalls)
		assert.Zero(t, client.listCalls)
	})
}

func TestCollectParamsForPolicy_RejectsInvalidScopeBeforeClientAccess(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		policy engineapi.PolicyScope
		err    string
	}{
		{name: "nil policy", err: "policy scope must not be nil"},
		{name: "typed nil policy", policy: (*kyvernov1.Policy)(nil), err: "policy scope must not be nil"},
		{name: "namespaced policy without namespace", policy: &kyvernov1.Policy{}, err: "policy namespace must not be empty"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, ref := range []struct {
				name     string
				paramRef *admissionregistrationv1.ParamRef
			}{
				{name: "named parameter", paramRef: &admissionregistrationv1.ParamRef{Name: "params", Namespace: "foreign-ns"}},
				{name: "selector parameters", paramRef: &admissionregistrationv1.ParamRef{Selector: &metav1.LabelSelector{}}},
			} {
				t.Run(ref.name, func(t *testing.T) {
					t.Parallel()
					client := &mockEngineClient{isNamespacedResp: true}
					params, err := CollectParamsForPolicy(context.Background(), client,
						&admissionregistrationv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
						ref.paramRef, "resource-ns", test.policy)
					assert.ErrorContains(t, err, test.err)
					assert.Nil(t, params)
					assert.Zero(t, client.discoveryCalls)
					assert.Zero(t, client.getCalls)
					assert.Zero(t, client.listCalls)
				})
			}
		})
	}
}

func TestCollectParamsForPolicy_NativeAdmissionPolicyScope(t *testing.T) {
	t.Parallel()
	for _, policy := range []struct {
		name  string
		scope engineapi.PolicyScope
	}{
		{name: "ValidatingAdmissionPolicy", scope: engineapi.NewValidatingAdmissionPolicy(&admissionregistrationv1.ValidatingAdmissionPolicy{})},
		{name: "MutatingAdmissionPolicy", scope: engineapi.NewMutatingAdmissionPolicy(&admissionregistrationv1beta1.MutatingAdmissionPolicy{})},
	} {
		t.Run(policy.name, func(t *testing.T) {
			t.Parallel()
			for _, test := range []struct {
				name              string
				kind              string
				isNamespaced      bool
				paramNamespace    string
				expectedNamespace string
			}{
				{name: "defaults to resource namespace", kind: "ConfigMap", isNamespaced: true, expectedNamespace: "resource-ns"},
				{name: "allows explicit namespace", kind: "ConfigMap", isNamespaced: true, paramNamespace: "params-ns", expectedNamespace: "params-ns"},
				{name: "allows cluster scoped parameters", kind: "Node"},
			} {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()
					client := &mockEngineClient{
						isNamespacedResp: test.isNamespaced,
						getResourceResp:  &unstructured.Unstructured{},
					}
					params, err := CollectParamsForPolicy(context.Background(), client,
						&admissionregistrationv1.ParamKind{APIVersion: "v1", Kind: test.kind},
						&admissionregistrationv1.ParamRef{Name: "params", Namespace: test.paramNamespace},
						"resource-ns", policy.scope)
					assert.NoError(t, err)
					assert.Len(t, params, 1)
					assert.Equal(t, 1, client.getCalls)
					assert.Equal(t, test.expectedNamespace, client.getNamespace)
				})
			}
		})
	}
}
