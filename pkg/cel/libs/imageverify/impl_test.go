package imageverify

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/stretchr/testify/assert"
)

var (
	cert = `-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----`

	ivpol = &v1alpha1.ImageValidatingPolicy{
		Spec: v1alpha1.ImageValidatingPolicySpec{
			Attestors: []v1alpha1.Attestor{
				{
					Name: "notary",
					Notary: &v1alpha1.Notary{
						Certs: &v1alpha1.StringOrExpression{
							Value: cert,
						},
					},
				},
			},
			Attestations: []v1alpha1.Attestation{
				{
					Name: "sbom",
					Referrer: &v1alpha1.Referrer{
						Type: "sbom/cyclone-dx",
					},
				},
			},
		},
	}
)

func Test_impl_verify_image_signature_string_stringarray(t *testing.T) {
	imgCtx, err := imagedataloader.NewImageContext(nil)
	assert.NoError(t, err)

	options := []cel.EnvOption{
		cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
		Lib(imgCtx, ivpol, nil),
	}
	env, err := cel.NewEnv(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)

	ast, issues := env.Compile(`verifyImageSignatures("ghcr.io/kyverno/test-verify-image:signed",[attestors.notary])`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)

	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	att := map[string]v1alpha1.Attestor{
		"notary": {
			Name: "notary",
			Notary: &v1alpha1.Notary{
				Certs: &v1alpha1.StringOrExpression{
					Value: cert,
				},
			},
		},
	}

	data := map[string]any{
		"attestors": att,
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	assert.Equal(t, out.Value(), int64(1))
}

func Test_impl_verify_image_attestations_string_string_stringarray(t *testing.T) {
	imgCtx, err := imagedataloader.NewImageContext(nil)
	assert.NoError(t, err)

	options := []cel.EnvOption{
		cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
		Lib(imgCtx, ivpol, nil),
	}
	env, err := cel.NewEnv(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)

	ast, issues := env.Compile(`verifyAttestationSignatures("ghcr.io/kyverno/test-verify-image:signed", "sbom" ,[attestors.notary])`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)

	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)

	att := map[string]v1alpha1.Attestor{
		"notary": {
			Name: "notary",
			Notary: &v1alpha1.Notary{
				Certs: &v1alpha1.StringOrExpression{
					Value: cert,
				},
			},
		},
	}

	data := map[string]any{
		"attestors": att,
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	assert.Equal(t, out.Value(), int64(1))
}
