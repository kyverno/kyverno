package utils

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
)

var podMatchSpec = policiesv1beta1.ImageValidatingPolicySpec{
	MatchConstraints: &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{"CREATE"},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
		}},
	},
}

func newTestScanner(t *testing.T) Scanner {
	t.Helper()
	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	dClient, err := dclient.NewFakeClient(scheme, map[schema.GroupVersionResource]string{})
	assert.NoError(t, err)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	return NewScanner(logging.GlobalLogger(), config.NewDefaultConfiguration(false), dClient, nil, nil, nil, nil)
}

// nilTypeConverterManager is a no-op TypeConverterManager for tests that exercise
// JSONPatch mutations, which do not need a type converter. Server-side-apply
// (ApplyConfiguration) mutations would need a real one backed by an OpenAPI client.
type nilTypeConverterManager struct{}

func (nilTypeConverterManager) GetTypeConverter(schema.GroupVersionKind) managedfields.TypeConverter {
	return nil
}

func (nilTypeConverterManager) Run(context.Context) {}

func newTestScannerWithTypeConverter(t *testing.T) Scanner {
	t.Helper()
	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	dClient, err := dclient.NewFakeClient(scheme, map[schema.GroupVersionResource]string{})
	assert.NoError(t, err)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	return NewScanner(logging.GlobalLogger(), config.NewDefaultConfiguration(false), dClient, nil, nil, nil, nilTypeConverterManager{})
}

func newDeploymentResource() (unstructured.Unstructured, schema.GroupVersionResource) {
	resource := unstructured.Unstructured{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	resource.SetName("test-deploy")
	resource.SetNamespace("test-ns")
	return resource, schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
}

func TestScanResource_NamespacedImageValidatingPolicy(t *testing.T) {
	nivp := &policiesv1beta1.NamespacedImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nivp", Namespace: "test-ns", ResourceVersion: "1"},
		Spec:       podMatchSpec,
	}
	policy := engineapi.NewNamespacedImageValidatingPolicy(nivp)

	resource, gvr := newDeploymentResource()
	results := newTestScanner(t).ScanResource(t.Context(), resource, gvr, "", nil, nil, nil, nil, policy)

	assert.Len(t, results, 1, "NamespacedImageValidatingPolicy must not be silently skipped by the scanner")
}

func TestScanResource_ImageValidatingPolicy(t *testing.T) {
	ivp := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ivp", ResourceVersion: "1"},
		Spec:       podMatchSpec,
	}
	policy := engineapi.NewImageValidatingPolicy(ivp)

	resource, gvr := newDeploymentResource()
	results := newTestScanner(t).ScanResource(t.Context(), resource, gvr, "", nil, nil, nil, nil, policy)

	assert.Len(t, results, 1, "ImageValidatingPolicy must not be silently skipped by the scanner")
}

func TestScanResource_RedactsSecretMessageEvents(t *testing.T) {
	fake := libs.NewFakeContextProvider()
	assert.NoError(t, fake.AddResource(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata":   map[string]any{"namespace": "default", "name": "db-credentials"},
			"data":       map[string]any{"password": "do-not-log"},
		},
	}))
	oldLibraryContext := libs.LibraryContext
	libs.LibraryContext = fake
	t.Cleanup(func() {
		libs.LibraryContext = oldLibraryContext
	})

	policy := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vpol"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: podMatchSpec.MatchConstraints,
			Validations: []admissionregistrationv1.Validation{{
				Expression:        "false",
				MessageExpression: `string(resource.Get("v1", "secrets", "default", "db-credentials").data.password)`,
			}},
		},
	})
	resource, gvr := newPodResource()
	results := newTestScanner(t).ScanResource(t.Context(), resource, gvr, "", nil, nil, nil, nil, policy)

	require.Len(t, results, 1)
	for _, result := range results {
		require.NoError(t, result.Error)
		require.NotNil(t, result.EngineResponse)
		assert.True(t, result.EngineResponse.RedactMessages)
		require.NotEmpty(t, result.EngineResponse.PolicyResponse.Rules)
		assert.Contains(t, result.EngineResponse.PolicyResponse.Rules[0].Message(), "do-not-log")

		events := webhookutils.GenerateEvents([]engineapi.EngineResponse{*result.EngineResponse}, false)
		require.Len(t, events, 2)
		for _, generatedEvent := range events {
			assert.Contains(t, generatedEvent.Message, "(redacted)")
			assert.NotContains(t, generatedEvent.Message, "do-not-log")
		}
	}
}

func TestHasSensitiveMessages(t *testing.T) {
	isSensitive := func(policy celengine.ValidatingPolicyResponse) bool {
		return policy.RedactMessages
	}
	assert.False(t, hasSensitiveMessages([]celengine.ValidatingPolicyResponse{{RedactMessages: false}}, isSensitive))
	assert.True(t, hasSensitiveMessages([]celengine.ValidatingPolicyResponse{
		{RedactMessages: false},
		{RedactMessages: true},
	}, isSensitive))
}

func newPodResource() (unstructured.Unstructured, schema.GroupVersionResource) {
	resource := unstructured.Unstructured{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	resource.SetName("test-pod")
	resource.SetNamespace("test-ns")
	resource.SetLabels(map[string]string{"app": "demo"})
	return resource, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
}

// TestScanResource_MutatingPolicy_MutateExisting reproduces the background-scan
// gap where a MutatingPolicy with mutateExisting enabled was silently skipped by
// the scanner: the scanner builds a single-policy provider and calls
// Fetch(ctx, false), which (before the fix) filtered mutate-existing policies
// out, so the engine evaluated nothing and the scan produced no rules.
func TestScanResource_MutatingPolicy_MutateExisting(t *testing.T) {
	enabled := true
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mp", ResourceVersion: "1"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &enabled,
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/scanned", value: "true"}]`,
				},
			}},
		},
	}
	policy := engineapi.NewMutatingPolicy(mp)

	resource, gvr := newPodResource()
	results := newTestScannerWithTypeConverter(t).ScanResource(t.Context(), resource, gvr, "", nil, nil, nil, nil, policy)

	assert.Len(t, results, 1, "mutateExisting MutatingPolicy must produce a scan result")
	for _, sr := range results {
		if assert.NotNil(t, sr.EngineResponse) {
			assert.NotEmpty(t, sr.EngineResponse.PolicyResponse.Rules, "mutateExisting MutatingPolicy must be evaluated by the scanner, not silently skipped")
		}
	}
}

func TestFilterMatchConditionSkips(t *testing.T) {
	tests := []struct {
		name          string
		rules         []engineapi.RuleResponse
		expectedCount int
	}{
		{
			name:          "empty input",
			rules:         []engineapi.RuleResponse{},
			expectedCount: 0,
		},
		{
			name: "skip with SkipReasonMatchConditions is filtered out",
			rules: []engineapi.RuleResponse{
				*engineapi.RuleSkip("rule-1", engineapi.Validation, "skip", nil).WithSkipReason(engineapi.SkipReasonMatchConditions),
			},
			expectedCount: 0,
		},
		{
			name: "skip without SkipReason is kept",
			rules: []engineapi.RuleResponse{
				*engineapi.RuleSkip("rule-1", engineapi.Validation, "skip", nil),
			},
			expectedCount: 1,
		},
		{
			name: "skip from PolicyException is kept",
			rules: []engineapi.RuleResponse{
				*engineapi.RuleSkip("exception", engineapi.Validation, "rule is skipped due to policy exception", nil),
			},
			expectedCount: 1,
		},
		{
			name: "pass and fail rules are never filtered",
			rules: []engineapi.RuleResponse{
				*engineapi.RulePass("rule-1", engineapi.Validation, "pass", nil),
				*engineapi.RuleFail("rule-2", engineapi.Validation, "fail", nil),
			},
			expectedCount: 2,
		},
		{
			name: "mixed -- only matchConditions skips are removed",
			rules: []engineapi.RuleResponse{
				*engineapi.RuleSkip("rule-1", engineapi.Validation, "skip", nil).WithSkipReason(engineapi.SkipReasonMatchConditions),
				*engineapi.RulePass("rule-2", engineapi.Validation, "pass", nil),
				*engineapi.RuleSkip("exception", engineapi.Validation, "policy exception", nil),
			},
			expectedCount: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterMatchConditionSkips(tt.rules)
			assert.Equal(t, tt.expectedCount, len(got))
		})
	}
}
