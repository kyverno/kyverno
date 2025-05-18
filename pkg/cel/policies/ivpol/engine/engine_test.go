package engine

import (
	"context"
	"encoding/json"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/imageverification/evaluator"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	signedImage   = "ghcr.io/kyverno/test-verify-image:signed"
	unsignedImage = "ghcr.io/kyverno/test-verify-image:unsigned"

	ivpol = &policiesv1alpha1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ivpol-notary",
		},
		Spec: policiesv1alpha1.ImageValidatingPolicySpec{
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
			EvaluationConfiguration: &policiesv1alpha1.EvaluationConfiguration{
				Mode: policiesv1alpha1.EvaluationModeKubernetes,
			},
			MatchImageReferences: []policiesv1alpha1.MatchImageReference{
				{
					Glob: "ghcr.io/*",
				},
			},
			ImageExtractors: []policiesv1alpha1.ImageExtractor{},
			Attestors: []policiesv1alpha1.Attestor{
				{
					Name: "notary",
					Notary: &policiesv1alpha1.Notary{
						Certs: &policiesv1alpha1.StringOrExpression{
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
			Attestations: []policiesv1alpha1.Attestation{
				{
					Name: "sbom",
					Referrer: &policiesv1alpha1.Referrer{
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

func Test_ImageVerifyEngine(t *testing.T) {
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
	engine := NewEngine(ProviderFunc(providerFunc), nsResolver, matching.NewMatcher(), nil, nil)

	resp, patches, err := engine.HandleMutating(context.Background(), engineRequest, nil)
	assert.NoError(t, err)
	assert.Equal(t, len(resp.Policies), 1)

	response := resp.Policies[0]
	assert.Equal(t, response.Result.Name(), "ivpol-notary")
	assert.Equal(t, response.Result.Status(), engineapi.RuleStatusPass)

	assert.Equal(t, len(patches), 2)
	outcomePatch := patches[1]
	data, ok := outcomePatch.Value.(string)
	assert.True(t, ok)

	var outcomes map[string]eval.ImageVerificationOutcome
	err = json.Unmarshal([]byte(data), &outcomes)
	assert.NoError(t, err)

	v, ok := outcomes["ivpol-notary"]
	assert.True(t, ok)
	assert.Equal(t, v.Status, engineapi.RuleStatusPass)
}
