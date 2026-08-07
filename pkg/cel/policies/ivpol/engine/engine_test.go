package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	admissionmatching "k8s.io/apiserver/pkg/admission/plugin/policy/matching"
)

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

	providerFunc = func(ctx context.Context) ([]Policy, error) {
		return []Policy{
			{
				Policy:  ivpol,
				Actions: sets.Set[admissionregistrationv1.ValidationAction]{admissionregistrationv1.Deny: sets.Empty{}},
			},
		}, nil
	}

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
		Request: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object: runtime.RawExtension{
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
	provider := ProviderFunc(func(context.Context) ([]Policy, error) {
		return []Policy{
			{
				Policy:  disabledIvpol,
				Actions: sets.Set[admissionregistrationv1.ValidationAction]{admissionregistrationv1.Deny: sets.Empty{}},
			},
		}, nil
	})
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
	provider := ProviderFunc(func(context.Context) ([]Policy, error) {
		return []Policy{
			{
				Policy:  policy,
				Actions: sets.Set[admissionregistrationv1.ValidationAction]{admissionregistrationv1.Deny: sets.Empty{}},
			},
		}, nil
	})
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
		Request: admissionv1.AdmissionRequest{
			Operation:       admissionv1.Update,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          runtime.RawExtension{Raw: []byte(podWithForgedOutcome)},
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
	provider := ProviderFunc(func(context.Context) ([]Policy, error) {
		return []Policy{
			{
				Policy:  policy,
				Actions: sets.Set[admissionregistrationv1.ValidationAction]{admissionregistrationv1.Deny: sets.Empty{}},
			},
		}, nil
	})
	podWithoutAnnotation := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-pod"},"spec":{"containers":[{"name":"main","image":"docker.io/library/busybox:latest"}]}}`
	engineRequest := engine.EngineRequest{
		Request: admissionv1.AdmissionRequest{
			Operation:       admissionv1.Update,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          runtime.RawExtension{Raw: []byte(podWithoutAnnotation)},
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
								Resources:   []string{"pods/ephemeralcontainers"},
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
	provider := ProviderFunc(func(context.Context) ([]Policy, error) {
		return []Policy{
			{
				Policy:  policy,
				Actions: sets.Set[admissionregistrationv1.ValidationAction]{admissionregistrationv1.Deny: sets.Empty{}},
			},
		}, nil
	})
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
		Request: admissionv1.AdmissionRequest{
			Operation:       admissionv1.Update,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			SubResource:     "ephemeralcontainers",
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          runtime.RawExtension{Raw: []byte(ephemeralUpdateWithForgedOutcome)},
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

func buildTestIvpol(t *testing.T, name string, shouldPass bool) Policy {
	t.Helper()
	expr := "true"
	if !shouldPass {
		expr = "false"
	}
	pol := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
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
			Validations: []admissionregistrationv1.Validation{
				{Expression: expr, Message: "test validation"},
			},
		},
	}
	return Policy{Policy: pol}
}

func buildBrokenTestIvpol(t *testing.T, name string) Policy {
	t.Helper()
	pol := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
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
			Validations: []admissionregistrationv1.Validation{
				{Expression: "this is not valid cel &&& (((", Message: "broken"},
			},
		},
	}
	return Policy{Policy: pol}
}

func testEngineRequest(t *testing.T, image string) engine.EngineRequest {
	t.Helper()

	podJSON := fmt.Sprintf(
		`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-pod"},"spec":{"containers":[{"name":"main","image":"%s"}]}}`,
		image,
	)

	return engine.EngineRequest{
		Request: admissionv1.AdmissionRequest{
			Operation:       admissionv1.Create,
			Kind:            metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
			RequestResource: &metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
			Object:          runtime.RawExtension{Raw: []byte(podJSON)},
		},
		Context: libs.NewFakeContextProvider(),
	}
}

func TestHandleValidation_ConcurrentEvaluation(t *testing.T) {
	libs.LibraryContext = libs.NewFakeContextProvider()
	const numPolicies = 20
	policies := make([]Policy, numPolicies)
	for i := range policies {
		policies[i] = buildTestIvpol(t, fmt.Sprintf("ivpol-pass-%02d", i), true)
	}
	provider := ProviderFunc(func(context.Context) ([]Policy, error) { return policies, nil })
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	resp, err := eng.HandleValidating(context.Background(), testEngineRequest(t, signedImage), nil)
	require.NoError(t, err)
	assert.Len(t, resp.Policies, numPolicies)
	for _, r := range resp.Policies {
		assert.Equal(t, engineapi.RuleStatusPass, r.Result.Status())
	}
}

func TestHandleValidation_ConcurrentEvaluationMixedResults(t *testing.T) {
	policies := []Policy{
		buildTestIvpol(t, "pass-1", true),
		buildTestIvpol(t, "fail-1", false),
		buildTestIvpol(t, "pass-2", true),
		buildTestIvpol(t, "fail-2", false),
		buildTestIvpol(t, "pass-3", true),
	}
	provider := ProviderFunc(func(context.Context) ([]Policy, error) { return policies, nil })
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	resp, err := eng.HandleValidating(context.Background(), testEngineRequest(t, signedImage), nil)
	require.NoError(t, err)
	assert.Len(t, resp.Policies, 5)

	passCount, failCount := 0, 0
	for _, r := range resp.Policies {
		switch r.Result.Status() {
		case engineapi.RuleStatusPass:
			passCount++
		case engineapi.RuleStatusFail:
			failCount++
		}
	}
	assert.Equal(t, 3, passCount)
	assert.Equal(t, 2, failCount)
}

func TestHandleValidation_EmptyPolicies(t *testing.T) {
	provider := ProviderFunc(func(context.Context) ([]Policy, error) { return nil, nil })
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	resp, err := eng.HandleValidating(context.Background(), testEngineRequest(t, signedImage), nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Policies)
}

func TestHandleValidation_LargePolicySet(t *testing.T) {
	const numPolicies = 100
	policies := make([]Policy, numPolicies)
	for i := range policies {
		policies[i] = buildTestIvpol(t, fmt.Sprintf("ivpol-%03d", i), i%3 != 0)
	}
	provider := ProviderFunc(func(context.Context) ([]Policy, error) { return policies, nil })
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	resp, err := eng.HandleValidating(context.Background(), testEngineRequest(t, signedImage), nil)
	require.NoError(t, err)
	assert.Len(t, resp.Policies, numPolicies, "all policies must be evaluated")

	passCount, failCount := 0, 0
	for _, r := range resp.Policies {
		if r.Result.Status() == engineapi.RuleStatusPass {
			passCount++
		} else {
			failCount++
		}
	}
	assert.Equal(t, 34, failCount)
	assert.Equal(t, 66, passCount)
}

func TestHandleValidation_DeterministicOrder(t *testing.T) {
	const numPolicies = 20
	const numRuns = 15

	policies := make([]Policy, numPolicies)
	for i := range policies {
		policies[i] = buildTestIvpol(t, fmt.Sprintf("ivpol-%02d", i), true)
	}
	provider := ProviderFunc(func(context.Context) ([]Policy, error) { return policies, nil })
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	var first []string
	for run := 0; run < numRuns; run++ {
		resp, err := eng.HandleValidating(context.Background(), testEngineRequest(t, signedImage), nil)
		require.NoError(t, err)
		require.Len(t, resp.Policies, numPolicies)

		names := make([]string, len(resp.Policies))
		for i, r := range resp.Policies {
			names[i] = r.Policy.GetName()
		}
		if run == 0 {
			first = names
		} else {
			assert.Equal(t, first, names, "response order changed on run %d", run)
		}
	}
}

func TestHandleValidation_CompileError(t *testing.T) {
	// one policy fails to compile; confirm it doesn't break the others
	policies := []Policy{
		buildTestIvpol(t, "good-1", true),
		buildBrokenTestIvpol(t, "broken-compile"),
		buildTestIvpol(t, "good-2", true),
	}
	provider := ProviderFunc(func(context.Context) ([]Policy, error) { return policies, nil })
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	resp, err := eng.HandleValidating(context.Background(), testEngineRequest(t, signedImage), nil)
	require.NoError(t, err) // compile errors are per-policy RuleErrors, not a hard failure
	assert.Len(t, resp.Policies, 3)

	var sawError bool
	for _, r := range resp.Policies {
		if r.Policy.GetName() == "broken-compile" {
			assert.Equal(t, engineapi.RuleStatusError, r.Result.Status())
			sawError = true
		}
	}
	assert.True(t, sawError, "expected broken-compile policy to surface a RuleError")
}

func TestHandleValidation_ConcurrentEvaluation_RaceStress(t *testing.T) {
	const numPolicies = 50
	const iterations = 10

	policies := make([]Policy, numPolicies)
	for i := range policies {
		policies[i] = buildTestIvpol(t, fmt.Sprintf("stress-%02d", i), i%2 == 0)
	}
	provider := ProviderFunc(func(context.Context) ([]Policy, error) { return policies, nil })
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	for iter := 0; iter < iterations; iter++ {
		resp, err := eng.HandleValidating(context.Background(), testEngineRequest(t, signedImage), nil)
		require.NoError(t, err)
		require.Len(t, resp.Policies, numPolicies)
	}
}

type fakeMatcher struct {
	matches bool
	err     error
}

func (f fakeMatcher) Match(_ admissionmatching.MatchCriteria, _ admission.Attributes, _ runtime.Object) (bool, error) {
	return f.matches, f.err
}

func TestFilterPolicies_NilMatcher(t *testing.T) {
	pol := buildTestIvpol(t, "p1", true)
	eng := &engineImpl{matcher: nil}

	results, filtered := eng.filterPolicies([]Policy{pol}, nil, nil, false)

	assert.Empty(t, results)
	assert.Equal(t, []Policy{pol}, filtered)
}

func TestFilterPolicies_MatchError(t *testing.T) {
	pol := buildTestIvpol(t, "p1", true)
	eng := &engineImpl{matcher: fakeMatcher{err: errors.New("boom")}}

	results, filtered := eng.filterPolicies([]Policy{pol}, nil, nil, false)

	require.Len(t, results, 1)
	assert.Equal(t, engineapi.RuleStatusError, results[0].Result.Status())
	assert.Empty(t, filtered)
}

func TestFilterPolicies_IncludeUnmatched(t *testing.T) {
	pol := buildTestIvpol(t, "p1", true)
	eng := &engineImpl{matcher: fakeMatcher{matches: false}}

	results, filtered := eng.filterPolicies([]Policy{pol}, nil, nil, true)

	require.Len(t, results, 1)
	assert.Equal(t, "p1", results[0].Policy.GetName())
	assert.Empty(t, filtered)
}
