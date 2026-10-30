package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

type fakeImageContext struct{}

func (fakeImageContext) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return nil
}

func (fakeImageContext) Get(context.Context, string, []remote.Option, []name.Option) (*imagedataloader.ImageData, error) {
	return &imagedataloader.ImageData{}, nil
}

type blockingImageContext struct {
	started chan struct{}
	release chan struct{}
}

func (c *blockingImageContext) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return nil
}

func (c *blockingImageContext) Get(ctx context.Context, image string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	select {
	case c.started <- struct{}{}:
	default:
	}

	select {
	case <-c.release:
		return &imagedataloader.ImageData{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

var (
	signedImage   = "ghcr.io/kyverno/test-verify-image:signed"
	unsignedImage = "ghcr.io/kyverno/test-verify-image:unsigned"

	ivpol = &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ivpol-notary",
		},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeKubernetes,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{
					Glob: "ghcr.io/*",
				},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{},
			Attestors: []policiesv1beta1.Attestor{
				{
					Name: "notary",
					Notary: &policiesv1beta1.Notary{
						Certs: &policiesv1beta1.StringOrExpression{
							Value: `-----BEGIN CERTIFICATE-----
MIIDTTCCAjWgAwIBAgIJAPI+zAzn4s0xMA0GCSqGSIb3DQEBCwUAMEwxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJXQTEQMA4GA1UEBwwHU2VhdHRsZTEPMA0GA1UECgwG
Tm90YXJ5MQ0wCwYDVQQDDAR0ZXN0MB4XDTIzMDUyMjIxMTUxOFoXDTMzMDUxOTIx
MTUxOFowTDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAldBMRAwDgYDVQQHDAdTZWF0
dGxlMQ8wDQYDVQQKDAZOb3RhcnkxDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQDNhTwv+QMk7jEHufFfIFlBjn2NiJaYPgL4eBS+
b+o37ve5Zn9nzRppV6kGsa161r9s2KkLXmJrojNy6vo9a6g6RtZ3F6xKiWLUmbAL
hVTCfYw/2n7xNlVMjyyUpE+7e193PF8HfQrfDFxe2JnX5LHtGe+X9vdvo2l41R6m
Iia04DvpMdG4+da2tKPzXIuLUz/FDb6IODO3+qsqQLwEKmmUee+KX+3yw8I6G1y0
Vp0mnHfsfutlHeG8gazCDlzEsuD4QJ9BKeRf2Vrb0ywqNLkGCbcCWF2H5Q80Iq/f
ETVO9z88R7WheVdEjUB8UrY7ZMLdADM14IPhY2Y+tLaSzEVZAgMBAAGjMjAwMAkG
A1UdEwQCMAAwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMDMA0G
CSqGSIb3DQEBCwUAA4IBAQBX7x4Ucre8AIUmXZ5PUK/zUBVOrZZzR1YE8w86J4X9
kYeTtlijf9i2LTZMfGuG0dEVFN4ae3CCpBst+ilhIndnoxTyzP+sNy4RCRQ2Y/k8
Zq235KIh7uucq96PL0qsF9s2RpTKXxyOGdtp9+HO0Ty5txJE2txtLDUIVPK5WNDF
ByCEQNhtHgN6V20b8KU2oLBZ9vyB8V010dQz0NRTDLhkcvJig00535/LUylECYAJ
5/jn6XKt6UYCQJbVNzBg/YPGc1RF4xdsGVDBben/JXpeGEmkdmXPILTKd9tZ5TC0
uOKpF5rWAruB5PCIrquamOejpXV9aQA/K2JQDuc0mcKz
-----END CERTIFICATE-----`,
						},
					},
				},
			},
			Attestations: []policiesv1beta1.Attestation{
				{
					Name: "sbom",
					Referrer: &policiesv1beta1.Referrer{
						Type: "sbom/cyclone-dx",
					},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "images.containers.map(i, parseImageReference(i).registry() == \"ghcr.io\" ).all(e, e)",
					Message:    "images are not from ghcr registry",
				},
				{
					Expression: "images.containers.map(image, verifyImageSignatures(image, [attestors.notary])).all(e, e > 0)",
					Message:    "failed to verify image with notary cert",
				},
				{
					Expression: "images.containers.map(image, verifyAttestationSignatures(image, attestations.sbom ,[attestors.notary])).all(e, e > 0)",
					Message:    "failed to verify attestation with notary cert",
				},
				{
					Expression: "images.containers.map(image, extractPayload(image, attestations.sbom).bomFormat == 'CycloneDX').all(e, e)",
					Message:    "sbom is not a cyclone dx sbom",
				},
			},
		},
	}

	providerFunc = singlePolicyProvider(ivpol)

	nsResolver = func(_ string) *corev1.Namespace {
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
	}

	pod = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
	   "name": "test-pod",
	   "namespace": ""
	},
	"spec": {
	   "containers": [
		  {
			 "name": "nginx",
			 "image": "ghcr.io/kyverno/test-verify-image:signed"
		  }
	   ]
	}
 }
`
)

func Test_ImageVerifyEngine_MutatingPinsDigest(t *testing.T) {
	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object: apiruntime.RawExtension{
				Raw: []byte(pod),
			},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
		Context: libs.NewFakeContextProvider(),
	}
	engine := NewEngine(ProviderFunc(providerFunc), nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	resp, patches, err := engine.HandleMutating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	assert.Empty(t, resp.Policies)
	if assert.Len(t, patches, 1) {
		assert.Equal(t, "replace", patches[0].Operation)
		assert.Equal(t, "/spec/containers/0/image", patches[0].Path)
		assert.Equal(t, signedImage, patches[0].Value.(string)[:len(signedImage)])
		assert.Contains(t, patches[0].Value, "@sha256:")
	}
}

func Test_ImageVerifyEngine_MutatingDisabled(t *testing.T) {
	falseVal := false
	disabledIvpol := ivpol.DeepCopy()
	disabledIvpol.Spec.ValidationConfigurations.MutateDigest = &falseVal
	provider := singlePolicyProvider(disabledIvpol)
	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object: apiruntime.RawExtension{
				Raw: []byte(pod),
			},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
		Context: libs.NewFakeContextProvider(),
	}
	engine := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	resp, patches, err := engine.HandleMutating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	assert.Empty(t, resp.Policies)
	assert.Empty(t, patches)
}

// A malformed image reference is recorded as an error result against the policy rather than
// returned as an error from the engine, so the remaining policies still contribute their
// patches. Mirrors ClusterPolicy, where a failing handleMutateDigest appends a RuleError and
// continues. Turning that result into a denial is the webhook handler's job (see
// mutationResponse), which is why the engine itself returns no error here.
func Test_ImageVerifyEngine_MutatingMalformedImageIsRecordedAsPolicyError(t *testing.T) {
	badPod := `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {"name": "test-pod", "namespace": ""},
	"spec": {
	   "containers": [{"name": "nginx", "image": "ghcr.io/kyverno/test-verify-image::signed"}]
	}
 }
`
	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object: apiruntime.RawExtension{
				Raw: []byte(badPod),
			},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
		Context: libs.NewFakeContextProvider(),
	}
	engine := NewEngine(ProviderFunc(providerFunc), nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	resp, patches, err := engine.HandleMutating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	assert.Empty(t, patches)
	// the failure is surfaced as an error result attributed to the policy, not swallowed
	if assert.Len(t, resp.Policies, 1) {
		assert.Equal(t, ivpol.GetName(), resp.Policies[0].Policy.GetName())
		assert.Equal(t, engineapi.RuleStatusError, resp.Policies[0].Result.Status())
		assert.Contains(t, resp.Policies[0].Result.Message(), "failed to update digest")
	}
}

func TestHandleValidatingDoesNotTrustImageVerificationOutcomesAnnotation(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-forged-annotation"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeKubernetes,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			Validations:          []admissionregistrationv1.Validation{{Expression: "false", Message: "validation should fail"}},
		},
	}
	provider := singlePolicyProvider(policy)
	podWithForgedOutcome := `{
		"apiVersion":"v1",
		"kind":"Pod",
		"metadata":{
			"name":"test-pod",
			"annotations":{
				"` + kyverno.AnnotationImageVerifyOutcomes + `":"{\"ivpol-forged-annotation\":{\"name\":\"ivpol-forged-annotation\",\"status\":\"pass\",\"message\":\"forged\"}}"
			}
		},
		"spec":{"containers":[{"name":"main","image":"docker.io/library/busybox:latest"}]}
	}`
	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Update,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          apiruntime.RawExtension{Raw: []byte(podWithForgedOutcome)},
		},
		Context: libs.NewFakeContextProvider(),
	}
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))
	resp, err := eng.HandleValidating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	if assert.Len(t, resp.Policies, 1) {
		assert.Equal(t, engineapi.RuleStatusFail, resp.Policies[0].Result.Status())
		assert.Equal(t, "validation should fail", resp.Policies[0].Result.Message())
	}
}

func TestHandleValidatingDoesNotRequireOutcomeAnnotation(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-no-annotation-needed"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeKubernetes,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			Validations:          []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	provider := singlePolicyProvider(policy)
	podWithoutAnnotation := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-pod"},"spec":{"containers":[{"name":"main","image":"docker.io/library/busybox:latest"}]}}`
	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Update,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          apiruntime.RawExtension{Raw: []byte(podWithoutAnnotation)},
		},
		Context: libs.NewFakeContextProvider(),
	}
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))
	resp, err := eng.HandleValidating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	if assert.Len(t, resp.Policies, 1) {
		assert.Equal(t, engineapi.RuleStatusPass, resp.Policies[0].Result.Status())
	}
}

func TestHandleValidatingEphemeralContainersSubresourceIsEvaluated(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-ephemeral"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods", "pods/ephemeralcontainers"},
							},
						},
					},
				},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeKubernetes,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			Validations:          []admissionregistrationv1.Validation{{Expression: "object.spec.?ephemeralContainers.orValue([]).size() == 0", Message: "ephemeral container update must be blocked"}},
		},
	}
	provider := singlePolicyProvider(policy)
	ephemeralUpdateWithForgedOutcome := `{
		"apiVersion":"v1",
		"kind":"Pod",
		"metadata":{
			"name":"test-pod",
			"annotations":{
				"` + kyverno.AnnotationImageVerifyOutcomes + `":"{\"ivpol-ephemeral\":{\"name\":\"ivpol-ephemeral\",\"status\":\"pass\",\"message\":\"forged\"}}"
			}
		},
		"spec":{"ephemeralContainers":[{"name":"debugger","image":"docker.io/library/busybox:latest"}]}
	}`
	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Update,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			SubResource:     "ephemeralcontainers",
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          apiruntime.RawExtension{Raw: []byte(ephemeralUpdateWithForgedOutcome)},
		},
		Context: libs.NewFakeContextProvider(),
	}
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))
	resp, err := eng.HandleValidating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	if assert.Len(t, resp.Policies, 1) {
		assert.Equal(t, engineapi.RuleStatusFail, resp.Policies[0].Result.Status())
		assert.Equal(t, "ephemeral container update must be blocked", resp.Policies[0].Result.Message())
	}
}

func TestHandleValidatingEphemeralContainersWithImagesVariable(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-images"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods", "pods/ephemeralcontainers"},
						},
					},
				}},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeKubernetes,
			},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(false),
				VerifyDigest: ptr.To(false),
				Required:     ptr.To(false),
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "*"}},
			Variables: []admissionregistrationv1.Variable{{
				Name:       "allImages",
				Expression: "images.?containers.orValue([]) + images.?initContainers.orValue([]) + images.?ephemeralContainers.orValue([])",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "variables.allImages.size() > 0 && variables.allImages.all(image, image.contains('signed'))",
				Message:    "All container images must be signed.",
			}},
		},
	}
	provider := singlePolicyProvider(policy)
	ephemeralUpdate := `{
		"apiVersion":"v1",
		"kind":"Pod",
		"metadata":{"name":"test-pod"},
		"spec":{
			"containers":[{"name":"main","image":"ghcr.io/kyverno/test-verify-image:signed"}],
			"ephemeralContainers":[{"name":"debugger","image":"docker.io/library/busybox:latest"}]
		}
	}`
	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Update,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			SubResource:     "ephemeralcontainers",
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          apiruntime.RawExtension{Raw: []byte(ephemeralUpdate)},
		},
		Context: libs.NewFakeContextProvider(),
	}
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return fakeImageContext{}, nil
	}
	resp, err := eng.HandleValidating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	if assert.Len(t, resp.Policies, 1) {
		assert.Equal(t, engineapi.RuleStatusFail, resp.Policies[0].Result.Status())
		assert.Equal(t, "All container images must be signed.", resp.Policies[0].Result.Message())
	}
}

func Test_ImageVerifyEngine_ValidatingPoliciesAreEvaluatedConcurrently(t *testing.T) {
	policy := func(name string) *policiesv1beta1.ImageValidatingPolicy {
		return &policiesv1beta1.ImageValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: policiesv1beta1.ImageValidatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{
									admissionregistrationv1.Create,
								},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
					},
				},
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
					Mode: policieskyvernoio.EvaluationModeKubernetes,
				},
				ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
					VerifyDigest: ptr.To(false),
					Required:     ptr.To(false),
				},
				MatchImageReferences: []policiesv1beta1.MatchImageReference{
					{
						Glob: "ghcr.io/*",
					},
				},
				ImageExtractors: []policiesv1beta1.ImageExtractor{
					{
						Name:       "containers",
						Expression: "object.spec.containers.map(e, e.image)",
					},
				},
				Validations: []admissionregistrationv1.Validation{
					{
						Expression: "images.containers.map(e, getImageData(e) != null).all(x, x)",
						Message:    "unexpected image registry",
					},
				},
			},
		}
	}

	policies := []Policy{
		{
			Policy: policy("ivpol-a"),
			Actions: sets.Set[admissionregistrationv1.ValidationAction]{
				admissionregistrationv1.Deny: sets.Empty{},
			},
		},
		{
			Policy: policy("ivpol-b"),
			Actions: sets.Set[admissionregistrationv1.ValidationAction]{
				admissionregistrationv1.Deny: sets.Empty{},
			},
		},
	}

	provider := multiPolicyProvider(
		policies[0].Policy,
		policies[1].Policy,
	)

	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind: metav1.GroupVersionKind{
				Group: "", Version: "v1", Kind: "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group: "", Version: "v1", Resource: "pods",
			},
			Object: apiruntime.RawExtension{
				Raw: []byte(pod),
			},
			RequestResource: &metav1.GroupVersionResource{
				Group: "", Version: "v1", Resource: "pods",
			},
		},
		Context: libs.NewFakeContextProvider(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eng := NewEngine(
		provider,
		nsResolver,
		matching.NewMatcher(),
		nil,
		nil,
		config.NewDefaultConfiguration(false),
	).(*engineImpl)

	imageContext := &blockingImageContext{
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}

	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return imageContext, nil
	}

	oldLibraryContext := libs.LibraryContext
	libs.LibraryContext = libs.NewFakeContextProvider()
	defer func() {
		libs.LibraryContext = oldLibraryContext
	}()

	engineRequest.Request.DryRun = ptr.To(false)

	type outcome struct {
		resp iveval.ImageVerifyEngineResponse
		err  error
	}
	outCh := make(chan outcome, 1)
	go func() {
		resp, err := eng.HandleValidating(ctx, engineRequest, nil)
		outCh <- outcome{resp, err}
	}()

	select {
	case <-imageContext.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first policy did not start image evaluation")
	}

	select {
	case <-imageContext.started:
		// Both policies reached image evaluation before either one was released.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second policy did not start while the first policy was blocked")
	}

	close(imageContext.release)

	select {
	case out := <-outCh:
		assert.NoError(t, out.err)
		assert.Len(t, out.resp.Policies, 2)
		for _, p := range out.resp.Policies {
			assert.Equal(t, engineapi.RuleStatusPass, p.Result.Status())
		}
	case <-time.After(time.Second):
		t.Fatal("engine did not complete")
	}
}

func multiPolicyProvider(policies ...policiesv1beta1.ImageValidatingPolicyLike) ProviderFunc {
	compiledPolicies := make([]Policy, 0, len(policies))

	for _, policy := range policies {
		compiled, errs := iveval.NewCompiler(nil).Compile(policy, nil)
		if len(errs) != 0 {
			panic(errs)
		}

		compiledPolicies = append(compiledPolicies, Policy{
			Policy:         policy,
			CompiledPolicy: compiled,
			Actions:        sets.New(admissionregistrationv1.Deny),
		})
	}

	return func(context.Context) ([]Policy, error) {
		return compiledPolicies, nil
	}
}

func singlePolicyProvider(policy policiesv1beta1.ImageValidatingPolicyLike) ProviderFunc {
	compiled, errs := iveval.NewCompiler(nil).Compile(policy, nil)
	if len(errs) != 0 {
		panic(errs)
	}
	return func(context.Context) ([]Policy, error) {
		return []Policy{{Policy: policy, CompiledPolicy: compiled, Actions: sets.New(admissionregistrationv1.Deny)}}, nil
	}
}

func Test_ImageVerifyEngine_DeterministicEvaluationOrdering(t *testing.T) {
	policy := func(name string, expr string) *policiesv1beta1.ImageValidatingPolicy {
		return &policiesv1beta1.ImageValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: policiesv1beta1.ImageValidatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{
									admissionregistrationv1.Create,
								},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
					},
				},
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
					Mode: policieskyvernoio.EvaluationModeKubernetes,
				},
				ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
					VerifyDigest: ptr.To(false),
					Required:     ptr.To(false),
				},
				MatchImageReferences: []policiesv1beta1.MatchImageReference{
					{
						Glob: "ghcr.io/*",
					},
				},
				ImageExtractors: []policiesv1beta1.ImageExtractor{
					{
						Name:       "containers",
						Expression: "object.spec.containers.map(e, e.image)",
					},
				},
				Validations: []admissionregistrationv1.Validation{
					{
						Expression: expr,
						Message:    "validation message",
					},
				},
			},
		}
	}

	namespacedPolicy := func(name, namespace, expr string) *policiesv1beta1.NamespacedImageValidatingPolicy {
		p := policy(name, expr)
		return &policiesv1beta1.NamespacedImageValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: p.Spec,
		}
	}

	compilePolicy := func(p policiesv1beta1.ImageValidatingPolicyLike) Policy {
		compiled, errs := iveval.NewCompiler(nil).Compile(p, nil)
		if len(errs) != 0 {
			panic(errs)
		}
		return Policy{
			Policy:         p,
			CompiledPolicy: compiled,
			Actions:        sets.New(admissionregistrationv1.Deny),
		}
	}

	pod := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default"
		},
		"spec": {
			"containers": [
				{
					"name": "app",
					"image": "ghcr.io/kyverno/test:v1"
				}
			]
		}
	}`

	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind: metav1.GroupVersionKind{
				Group: "", Version: "v1", Kind: "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group: "", Version: "v1", Resource: "pods",
			},
			Object: apiruntime.RawExtension{
				Raw: []byte(pod),
			},
			RequestResource: &metav1.GroupVersionResource{
				Group: "", Version: "v1", Resource: "pods",
			},
			DryRun: ptr.To(false),
		},
		Context: libs.NewFakeContextProvider(),
	}

	passExpr := "images.containers.map(e, getImageData(e) != null).all(x, x)"
	failExpr := "false"

	testCases := []struct {
		name          string
		policies      []Policy
		expectedNames []string
		expectedStats []engineapi.RuleStatus
	}{
		{
			name: "all_pass",
			policies: []Policy{
				compilePolicy(policy("ivpol-1-pass", passExpr)),
				compilePolicy(policy("ivpol-2-pass", passExpr)),
				compilePolicy(policy("ivpol-3-pass", passExpr)),
				compilePolicy(policy("ivpol-4-pass", passExpr)),
			},
			expectedNames: []string{"ivpol-1-pass", "ivpol-2-pass", "ivpol-3-pass", "ivpol-4-pass"},
			expectedStats: []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusPass, engineapi.RuleStatusPass, engineapi.RuleStatusPass},
		},
		{
			name: "mixed_pass_and_failures",
			policies: []Policy{
				compilePolicy(policy("ivpol-1-pass", passExpr)),
				compilePolicy(policy("ivpol-2-fail", failExpr)),
				compilePolicy(policy("ivpol-3-pass", passExpr)),
				compilePolicy(policy("ivpol-4-fail", failExpr)),
			},
			expectedNames: []string{"ivpol-1-pass", "ivpol-2-fail", "ivpol-3-pass", "ivpol-4-fail"},
			expectedStats: []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusFail, engineapi.RuleStatusPass, engineapi.RuleStatusFail},
		},
		{
			name: "duplicate_policy_names_deduplicated_to_first_position",
			policies: []Policy{
				compilePolicy(policy("ivpol-alpha", passExpr)),
				compilePolicy(policy("ivpol-alpha", passExpr)),
				compilePolicy(policy("ivpol-beta", passExpr)),
				compilePolicy(policy("ivpol-alpha", passExpr)),
			},
			expectedNames: []string{"ivpol-alpha", "ivpol-beta"},
			expectedStats: []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusPass},
		},
		{
			name: "duplicate_policy_names_failure_takes_precedence",
			policies: []Policy{
				compilePolicy(policy("ivpol-alpha", passExpr)),
				compilePolicy(policy("ivpol-alpha", failExpr)),
				compilePolicy(policy("ivpol-beta", passExpr)),
				compilePolicy(policy("ivpol-alpha", passExpr)),
			},
			expectedNames: []string{"ivpol-alpha", "ivpol-beta"},
			expectedStats: []engineapi.RuleStatus{engineapi.RuleStatusFail, engineapi.RuleStatusPass},
		},
		{
			name: "nil_compiled_policy_error_preserves_order",
			policies: []Policy{
				compilePolicy(policy("ivpol-1-pass", passExpr)),
				{
					Policy:         policy("ivpol-2-nil", passExpr),
					CompiledPolicy: nil,
					Actions:        sets.New(admissionregistrationv1.Deny),
				},
				compilePolicy(policy("ivpol-3-pass", passExpr)),
			},
			expectedNames: []string{"ivpol-1-pass", "ivpol-2-nil", "ivpol-3-pass"},
			expectedStats: []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusError, engineapi.RuleStatusPass},
		},
		{
			name: "same_name_in_different_namespaces_not_deduplicated",
			policies: []Policy{
				compilePolicy(namespacedPolicy("shared-name", "tenant-a", passExpr)),
				compilePolicy(namespacedPolicy("shared-name", "tenant-b", failExpr)),
			},
			expectedNames: []string{"shared-name", "shared-name"},
			expectedStats: []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusFail},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := ProviderFunc(func(context.Context) ([]Policy, error) {
				return tc.policies, nil
			})

			eng := NewEngine(
				provider,
				nsResolver,
				matching.NewMatcher(),
				nil,
				nil,
				config.NewDefaultConfiguration(false),
			).(*engineImpl)

			eng.newImageContext = func() (imagedataloader.ImageContext, error) {
				return fakeImageContext{}, nil
			}

			oldLibraryContext := libs.LibraryContext
			libs.LibraryContext = libs.NewFakeContextProvider()
			defer func() {
				libs.LibraryContext = oldLibraryContext
			}()

			for i := 0; i < 50; i++ {
				resp, err := eng.HandleValidating(context.Background(), engineRequest, nil)
				assert.NoError(t, err)
				require.Len(t, resp.Policies, len(tc.expectedNames))
				for j, expected := range tc.expectedNames {
					assert.Equal(t, expected, resp.Policies[j].Policy.GetName(), "case %s, iteration %d: policy name mismatch at index %d", tc.name, i, j)
					assert.Equal(t, tc.expectedStats[j], resp.Policies[j].Result.Status(), "case %s, iteration %d: policy status mismatch at index %d", tc.name, i, j)
				}
			}
		})
	}
}

func Test_ImageVerifyEngine_PolicyExceptionRuleType(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-images",
		},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeKubernetes,
			},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				VerifyDigest: ptr.To(false),
				Required:     ptr.To(false),
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			ImageExtractors: []policiesv1beta1.ImageExtractor{{
				Name:       "containers",
				Expression: "object.spec.containers.map(e, e.image)",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "images.containers.map(e, getImageData(e) != null).all(x, x)",
				Message:    "fail",
			}},
		},
	}

	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-pod",
			Namespace: "default",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{
					Name: "check-images",
					Kind: "ImageValidatingPolicy",
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "match-all",
					Expression: "true",
				},
			},
		},
	}

	compiled, errs := iveval.NewCompiler(nil).Compile(policy, []*policiesv1beta1.PolicyException{polex})
	assert.Empty(t, errs)

	provider := ProviderFunc(func(context.Context) ([]Policy, error) {
		return []Policy{{
			Policy:         policy,
			CompiledPolicy: compiled,
			Exceptions:     []*policiesv1beta1.PolicyException{polex},
			Actions:        sets.New(admissionregistrationv1.Deny),
		}}, nil
	})

	pod := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default"
		},
		"spec": {
			"containers": [
				{
					"name": "app",
					"image": "ghcr.io/kyverno/test:v1"
				}
			]
		}
	}`

	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind: metav1.GroupVersionKind{
				Group: "", Version: "v1", Kind: "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group: "", Version: "v1", Resource: "pods",
			},
			Object: apiruntime.RawExtension{
				Raw: []byte(pod),
			},
			RequestResource: &metav1.GroupVersionResource{
				Group: "", Version: "v1", Resource: "pods",
			},
			DryRun: ptr.To(false),
		},
		Context: libs.NewFakeContextProvider(),
	}

	eng := NewEngine(
		provider,
		nsResolver,
		matching.NewMatcher(),
		nil,
		nil,
		config.NewDefaultConfiguration(false),
	).(*engineImpl)

	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return fakeImageContext{}, nil
	}

	resp, err := eng.HandleValidating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	if assert.Len(t, resp.Policies, 1) {
		assert.Equal(t, engineapi.RuleStatusSkip, resp.Policies[0].Result.Status())
		assert.Equal(t, engineapi.ImageVerify, resp.Policies[0].Result.RuleType(), "ruleType for IVPOL exception must be ImageVerify, not Validation")
	}
}

func TestHandleValidating_OverlappingAutogenConfigurations_DenialNotMaskedByPass(t *testing.T) {
	buildPolicy := func(expr string) *policiesv1beta1.ImageValidatingPolicy {
		return &policiesv1beta1.ImageValidatingPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ImageValidatingPolicy",
				APIVersion: policiesv1beta1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "check-deployment-autogen",
			},
			Spec: policiesv1beta1.ImageValidatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					}},
				},
				AutogenConfiguration: &policiesv1beta1.ImageValidatingPolicyAutogenConfiguration{
					PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
						Controllers: []string{"deployments", "deployments.v1.apps"},
					},
				},
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
					Mode: policieskyvernoio.EvaluationModeKubernetes,
				},
				ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
					VerifyDigest: ptr.To(false),
					Required:     ptr.To(false),
				},
				MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
				ImageExtractors: []policiesv1beta1.ImageExtractor{{
					Name:       "containers",
					Expression: "['ghcr.io/kyverno/test:v1']",
				}},
				Validations: []admissionregistrationv1.Validation{{
					Expression: expr,
					Message:    "validation failed",
				}},
			},
		}
	}

	deployment := `{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "test-deployment",
			"namespace": "default"
		},
		"spec": {
			"template": {
				"metadata": {
					"name": "test-pod"
				},
				"spec": {
					"containers": [
						{
							"name": "app",
							"image": "ghcr.io/kyverno/test:v1"
						}
					]
				}
			}
		}
	}`

	engineRequest := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind: metav1.GroupVersionKind{
				Group: "apps", Version: "v1", Kind: "Deployment",
			},
			Resource: metav1.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			Object: apiruntime.RawExtension{
				Raw: []byte(deployment),
			},
			RequestResource: &metav1.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			DryRun: ptr.To(false),
		},
		Context: libs.NewFakeContextProvider(),
	}

	cases := []struct {
		name       string
		expr       string
		mutate     func(*policiesv1beta1.ImageValidatingPolicy)
		exceptions []*policiesv1beta1.PolicyException
		traced     bool
		wantStatus engineapi.RuleStatus
		wantMsg    string
	}{
		{
			name:       "defaults_fails_and_extract_passes",
			expr:       "object.kind == 'Pod'",
			wantStatus: engineapi.RuleStatusFail,
			wantMsg:    "validation failed",
		},
		{
			name:       "defaults_passes_and_extract_fails",
			expr:       "object.kind == 'Deployment'",
			wantStatus: engineapi.RuleStatusFail,
			wantMsg:    "validation failed (pod template at spec.template)",
		},
		{
			name: "untraced_defaults_fails_and_extract_skipped",
			expr: "false",
			mutate: func(p *policiesv1beta1.ImageValidatingPolicy) {
				p.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
					Name:       "is-deployment",
					Expression: "object.kind == 'Deployment'",
				}}
			},
			traced:     false,
			wantStatus: engineapi.RuleStatusFail,
			wantMsg:    "validation failed",
		},
		{
			name: "traced_defaults_fails_and_extract_skipped",
			expr: "false",
			mutate: func(p *policiesv1beta1.ImageValidatingPolicy) {
				p.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
					Name:       "is-deployment",
					Expression: "object.kind == 'Deployment'",
				}}
			},
			traced:     true,
			wantStatus: engineapi.RuleStatusFail,
			wantMsg:    "validation failed",
		},
		{
			name: "overlapping_variants_with_required_enabled_preserves_retained_failure",
			expr: "object.kind == 'Pod'",
			mutate: func(p *policiesv1beta1.ImageValidatingPolicy) {
				p.Spec.ValidationConfigurations.Required = ptr.To(true)
			},
			wantStatus: engineapi.RuleStatusFail,
			wantMsg:    "validation failed",
		},
		{
			name: "policy_named_match_defaults_fails_and_extract_exempted_by_exception",
			expr: "object.kind == 'Pod'",
			mutate: func(p *policiesv1beta1.ImageValidatingPolicy) {
				p.SetName("match")
			},
			exceptions: []*policiesv1beta1.PolicyException{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "exempt-pod",
					Namespace: "default",
				},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Name: "match",
						Kind: "ImageValidatingPolicy",
					}},
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "match-pod",
						Expression: "object.kind == 'Pod'",
					}},
				},
			}},
			wantStatus: engineapi.RuleStatusFail,
			wantMsg:    "validation failed",
		},
		{
			name: "policy_named_match_defaults_passes_and_extract_exempted_with_unverified_image_fails_required",
			expr: "object.kind == 'Deployment'",
			mutate: func(p *policiesv1beta1.ImageValidatingPolicy) {
				p.SetName("match")
				p.Spec.ValidationConfigurations.Required = ptr.To(true)
			},
			exceptions: []*policiesv1beta1.PolicyException{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "exempt-pod",
					Namespace: "default",
				},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Name: "match",
						Kind: "ImageValidatingPolicy",
					}},
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "match-pod",
						Expression: "object.kind == 'Pod'",
					}},
				},
			}},
			wantStatus: engineapi.RuleStatusFail,
			wantMsg:    "image ghcr.io/kyverno/test:v1 is not verified: no policy performed a signature or attestation check on it",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol := buildPolicy(tc.expr)
			if tc.mutate != nil {
				tc.mutate(pol)
			}
			prov, err := NewProvider(iveval.NewCompilerWithTrace(nil, tc.traced), []policiesv1beta1.ImageValidatingPolicyLike{pol}, tc.exceptions)
			require.NoError(t, err)

			eng := NewEngine(
				prov,
				nsResolver,
				matching.NewMatcher(),
				nil,
				nil,
				config.NewDefaultConfiguration(false),
			).(*engineImpl)

			eng.newImageContext = func() (imagedataloader.ImageContext, error) {
				return fakeImageContext{}, nil
			}

			resp, err := eng.HandleValidating(context.Background(), engineRequest, nil)
			require.NoError(t, err)
			require.Len(t, resp.Policies, 1)
			assert.Equal(t, tc.wantStatus, resp.Policies[0].Result.Status())
			if tc.wantMsg != "" {
				assert.Equal(t, tc.wantMsg, resp.Policies[0].Result.Message())
			}
			if tc.traced {
				require.NotNil(t, resp.Policies[0].Trace, "traced run must retain trace")
			}
		})
	}
}

func TestShouldReplaceResponse(t *testing.T) {
	emptyResp := eval.ImageVerifyPolicyResponse{}
	skipResp := eval.ImageVerifyPolicyResponse{Result: *engineapi.RuleSkip("rule", engineapi.ImageVerify, "skipped", nil)}
	passResp := eval.ImageVerifyPolicyResponse{Result: *engineapi.RulePass("rule", engineapi.ImageVerify, "passed", nil)}
	failResp := eval.ImageVerifyPolicyResponse{Result: *engineapi.RuleFail("rule", engineapi.ImageVerify, "failed", nil)}
	errResp := eval.ImageVerifyPolicyResponse{Result: *engineapi.RuleError("rule", engineapi.ImageVerify, "error", errors.New("err"), nil)}

	cases := []struct {
		name        string
		existing    eval.ImageVerifyPolicyResponse
		candidate   eval.ImageVerifyPolicyResponse
		wantReplace bool
	}{
		{"empty_replaced_by_empty", emptyResp, emptyResp, true},
		{"empty_replaced_by_skip", emptyResp, skipResp, true},
		{"empty_replaced_by_pass", emptyResp, passResp, true},
		{"empty_replaced_by_fail", emptyResp, failResp, true},
		{"empty_replaced_by_error", emptyResp, errResp, true},

		{"skip_not_replaced_by_empty", skipResp, emptyResp, false},
		{"skip_replaced_by_skip", skipResp, skipResp, true},
		{"skip_replaced_by_pass", skipResp, passResp, true},
		{"skip_replaced_by_fail", skipResp, failResp, true},
		{"skip_replaced_by_error", skipResp, errResp, true},

		{"pass_not_replaced_by_empty", passResp, emptyResp, false},
		{"pass_not_replaced_by_skip", passResp, skipResp, false},
		{"pass_replaced_by_pass", passResp, passResp, true},
		{"pass_replaced_by_fail", passResp, failResp, true},
		{"pass_replaced_by_error", passResp, errResp, true},

		{"fail_not_replaced_by_empty", failResp, emptyResp, false},
		{"fail_not_replaced_by_skip", failResp, skipResp, false},
		{"fail_not_replaced_by_pass", failResp, passResp, false},
		{"fail_replaced_by_fail", failResp, failResp, true},
		{"fail_replaced_by_error", failResp, errResp, true},

		{"error_not_replaced_by_empty", errResp, emptyResp, false},
		{"error_not_replaced_by_skip", errResp, skipResp, false},
		{"error_not_replaced_by_pass", errResp, passResp, false},
		{"error_replaced_by_fail", errResp, failResp, true},
		{"error_replaced_by_error", errResp, errResp, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldReplaceResponse(tc.existing, tc.candidate)
			assert.Equal(t, tc.wantReplace, got)
		})
	}
}
