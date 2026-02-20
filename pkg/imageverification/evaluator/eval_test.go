package eval

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

var (
	obj = func(image string) map[string]any {
		return map[string]any{
			"foo": map[string]string{
				"bar": image,
			},
		}
	}

	signedImage   = "ghcr.io/kyverno/test-verify-image:signed"
	unsignedImage = "ghcr.io/kyverno/test-verify-image:unsigned"

	ivpol = &policiesv1beta1.ImageValidatingPolicy{
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{
					Glob: "ghcr.io/*",
				},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{
					Name:       "bar",
					Expression: "[object.foo.bar]",
				},
			},
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
					Expression: "images.bar.map(image, verifyImageSignatures(image, [attestors.notary])).all(e, e > 0)",
					Message:    "failed to verify image with notary cert",
				},
				{
					Expression: "images.bar.map(image, verifyAttestationSignatures(image, attestations.sbom ,[attestors.notary])).all(e, e > 0)",
					Message:    "failed to verify attestation with notary cert",
				},
				{
					Expression: "images.bar.map(image, extractPayload(image, attestations.sbom).bomFormat == 'CycloneDX').all(e, e)",
					Message:    "sbom is not a cyclone dx sbom",
				},
			},
		},
	}
)

func Test_Eval(t *testing.T) {
	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: ivpol}}, obj(signedImage), nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, len(result) == 1)
	assert.True(t, result[ivpol.Name].Result)

	result, err = Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: ivpol}}, obj(unsignedImage), nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, len(result) == 1)
	assert.False(t, result[ivpol.Name].Result)
	assert.Equal(t, result[ivpol.Name].Message, "failed to verify image with notary cert")
}

func Test_Eval_FilterNonMatchingImages(t *testing.T) {
	nonMatchingImage := "docker.io/library/nginx:latest"
	policyWithFilter := &policiesv1beta1.ImageValidatingPolicy{
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{
					Glob: "ghcr.io/*",
				},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{
					Name:       "bar",
					Expression: "[object.foo.bar]",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "size(images.bar) == 0",
					Message:    "non-matching images should be filtered out",
				},
			},
		},
	}

	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: policyWithFilter}}, obj(nonMatchingImage), nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, len(result) == 1)
	assert.True(t, result[policyWithFilter.Name].Result)
	assert.True(t, result[policyWithFilter.Name].Skipped, "policy should be skipped when no images match")
}

func Test_Eval_PreserveEmptyImageKeys(t *testing.T) {
	nonMatchingImage := "docker.io/library/nginx:latest"
	policyWithMultipleExtractors := &policiesv1beta1.ImageValidatingPolicy{
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{
					Glob: "ghcr.io/*",
				},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{
					Name:       "containers",
					Expression: "object.spec.containers.map(c, c.image)",
				},
				{
					Name:       "initContainers",
					Expression: "object.spec.initContainers.orValue([]).map(c, c.image)",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "has(images.containers) && has(images.initContainers)",
					Message:    "all image keys should be preserved even when empty",
				},
			},
		},
	}

	podObj := map[string]any{
		"spec": map[string]any{
			"containers": []map[string]any{
				{"image": nonMatchingImage},
			},
			"initContainers": []map[string]any{},
		},
	}

	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: policyWithMultipleExtractors}}, podObj, nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, len(result) == 1)
	assert.True(t, result[policyWithMultipleExtractors.Name].Result, "validation should pass because keys exist")
	assert.True(t, result[policyWithMultipleExtractors.Name].Skipped, "policy should be skipped when no images match")
}

func Test_Eval_MixedMatchingAndNonMatchingImages(t *testing.T) {
	matchingImage := "ghcr.io/kyverno/test-verify-image:signed"
	nonMatchingImage := "docker.io/library/nginx:latest"
	policyWithFilter := &policiesv1beta1.ImageValidatingPolicy{
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{
					Glob: "ghcr.io/*",
				},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{
					Name:       "bar",
					Expression: "[object.foo.bar, object.foo.baz]",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "size(images.bar) == 1 && images.bar[0] == 'ghcr.io/kyverno/test-verify-image:signed'",
					Message:    "only matching images should be in context",
				},
			},
		},
	}

	mixedObj := map[string]any{
		"foo": map[string]string{
			"bar": matchingImage,
			"baz": nonMatchingImage,
		},
	}

	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: policyWithFilter}}, mixedObj, nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, len(result) == 1)
	assert.NotNil(t, result[policyWithFilter.Name])
	assert.False(t, result[policyWithFilter.Name].Skipped, "policy should not be skipped when images match")
}
