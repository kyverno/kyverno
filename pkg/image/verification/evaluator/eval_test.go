package evaluator

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

var nonMatchingImage = "docker.io/library/nginx:latest"

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
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				VerifyDigest: func() *bool { b := false; return &b }(),
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

func Test_Eval_NonMatchingImage(t *testing.T) {
	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: ivpol}}, obj(nonMatchingImage), nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, result[ivpol.Name].Result)
}

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

func Test_Eval_VerifyDigest_ImageWithoutDigest_Fails(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				VerifyDigest: func() *bool { b := true; return &b }(),
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{Glob: "ghcr.io/*"},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{Name: "bar", Expression: "[object.foo.bar]"},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "true"},
			},
		},
	}

	imageWithoutDigest := "ghcr.io/kyverno/test-verify-image:latest"
	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: policy}}, obj(imageWithoutDigest), nil, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result[policy.Name])
	assert.False(t, result[policy.Name].Result)
	assert.Contains(t, result[policy.Name].Message, "does not have a digest")
}

// A policy whose expressions never perform a cryptographic check must not be
// treated as having verified the image. This is the gap
// validationConfigurations.required closes: such a policy passes all of its
// validations while nothing was actually verified, so before required was
// enforced it silently admitted every image while looking like it enforced
// signature verification.
func Test_Eval_RequiredDeniesPolicyThatVerifiesNothing(t *testing.T) {
	p := ivpol.DeepCopy()
	p.Name = "verifies-nothing"
	p.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}

	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: p}}, obj(signedImage), nil, nil, nil)
	assert.NoError(t, err)
	require.NotNil(t, result[p.Name])
	assert.False(t, result[p.Name].Result, "an image that no expression verified must not be admitted")
	assert.Contains(t, result[p.Name].Message, signedImage)
	assert.Contains(t, result[p.Name].Message, "no policy performed a signature or attestation check")
}

// Opting out has to keep working, otherwise policies that legitimately only
// inspect image metadata would start being denied.
func Test_Eval_RequiredDisabledAllowsPolicyThatVerifiesNothing(t *testing.T) {
	p := ivpol.DeepCopy()
	p.Name = "verifies-nothing-required-off"
	p.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}
	p.Spec.ValidationConfigurations.Required = ptr.To(false)

	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: p}}, obj(signedImage), nil, nil, nil)
	assert.NoError(t, err)
	require.NotNil(t, result[p.Name])
	assert.True(t, result[p.Name].Result)
}

// An expression that tolerates a zero verification count reports success without
// anything having been verified, which required must still catch.
func Test_Eval_RequiredDeniesExpressionToleratingZeroVerifications(t *testing.T) {
	p := ivpol.DeepCopy()
	p.Name = "tolerates-zero"
	p.Spec.Validations = []admissionregistrationv1.Validation{
		{Expression: "images.bar.map(image, verifyImageSignatures(image, [attestors.notary])).all(e, e >= 0)"},
	}

	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: p}}, obj(unsignedImage), nil, nil, nil)
	assert.NoError(t, err)
	require.NotNil(t, result[p.Name])
	assert.False(t, result[p.Name].Result, "a passing expression must not override a failed verification")
	assert.Contains(t, result[p.Name].Message, "failed signature or attestation verification")
}
