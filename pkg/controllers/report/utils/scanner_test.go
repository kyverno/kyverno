package utils

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
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


var _ imageverifycache.Client = (*recordingImageVerifyCache)(nil)

type recordingImageVerifyCache struct {
	getCalls int
	lastRef  string
	hit      bool
}

func (c *recordingImageVerifyCache) Set(string, []policiesv1beta1.Attestor, *policiesv1beta1.Attestation, map[string][]byte) (bool, error) {
	return true, nil
}

func (c *recordingImageVerifyCache) Get(imageRef string, _ []policiesv1beta1.Attestor, _ *policiesv1beta1.Attestation) (bool, map[string][]byte, error) {
	c.getCalls++
	c.lastRef = imageRef
	return c.hit, nil, nil
}

func TestScanResource_ImageValidatingPolicy_UsesSuppliedImageVerifyCache(t *testing.T) {
	const testImage = "registry.invalid/kyverno/test@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105"
	verifyDigest := false
	required := false
	ivp := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-ivp-cache",
			UID:             "test-uid",
			ResourceVersion: "1",
		},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints:        podMatchSpec.MatchConstraints,
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeKubernetes},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				VerifyDigest: &verifyDigest,
				Required:     &required,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "registry.invalid/*"}},
			ImageExtractors:      []policiesv1beta1.ImageExtractor{{Name: "containers", Expression: "object.spec.containers.map(e, e.image)"}},
			Attestors: []policiesv1beta1.Attestor{{
				Name: "notary",
				Notary: &policiesv1beta1.Notary{
					Certs: &policiesv1beta1.StringOrExpression{Value: "test-cert"},
				},
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "images.containers.map(image, verifyImageSignatures(image, [attestors.notary])).all(e, e > 0)",
				Message:    "signature verification failed",
			}},
		},
	}
	policy := engineapi.NewImageValidatingPolicy(ivp)

	resource, gvr := newPodResource()
	resource.Object["spec"] = map[string]any{
		"containers": []any{
			map[string]any{
				"name":  "app",
				"image": testImage,
			},
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	dClient, err := dclient.NewFakeClient(scheme, map[schema.GroupVersionResource]string{})
	assert.NoError(t, err)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	recCache := &recordingImageVerifyCache{hit: true}
	s := NewScanner(logging.GlobalLogger(), config.NewDefaultConfiguration(false), dClient, nil, nil, nil, nil, recCache)
	results := s.ScanResource(t.Context(), resource, gvr, "", nil, nil, nil, nil, policy)

	assert.Equal(t, 1, recCache.getCalls, "ScanResource must query the supplied ImageVerifyCacheClient")
	assert.Equal(t, testImage, recCache.lastRef)
	assert.Len(t, results, 1)
	for _, sr := range results {
		assert.NoError(t, sr.Error)
		if assert.NotNil(t, sr.EngineResponse) && assert.Len(t, sr.EngineResponse.PolicyResponse.Rules, 1) {
			assert.Equal(t, engineapi.RuleStatusPass, sr.EngineResponse.PolicyResponse.Rules[0].Status())
		}
	}
}
