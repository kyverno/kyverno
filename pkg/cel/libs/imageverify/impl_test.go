package imageverify

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/cosign"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/notary"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	ivpol = &v1beta1.ImageValidatingPolicy{
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors: []v1beta1.Attestor{
				{
					Name: "notary",
					Notary: &v1beta1.Notary{
						Certs: &v1beta1.StringOrExpression{
							Value: cert,
						},
					},
				},
			},
			Attestations: []v1beta1.Attestation{
				{
					Name: "sbom",
					Referrer: &v1beta1.Referrer{
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
		Lib(nil, imgCtx, ivpol, nil, logr.Discard(), nil),
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
	att := map[string]v1beta1.Attestor{
		"notary": {
			Name: "notary",
			Notary: &v1beta1.Notary{
				Certs: &v1beta1.StringOrExpression{
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
		Lib(nil, imgCtx, ivpol, nil, logr.Discard(), nil),
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

	att := map[string]v1beta1.Attestor{
		"notary": {
			Name: "notary",
			Notary: &v1beta1.Notary{
				Certs: &v1beta1.StringOrExpression{
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

func Test_impl_verify_image_signature_cache_hit(t *testing.T) {
	attestors := []v1beta1.Attestor{
		{
			Name: "notary",
			Notary: &v1beta1.Notary{
				Certs: &v1beta1.StringOrExpression{
					Value: cert,
				},
			},
		},
	}
	pol := &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cache-policy",
			UID:             "test-uid",
			ResourceVersion: "1",
		},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors: attestors,
		},
	}
	image := "ghcr.io/kyverno/test-verify-image:signed"

	ivCache, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(0),
		imageverifycache.WithTTLDuration(0),
	)
	assert.NoError(t, err)

	// imgCtx is left nil on purpose: if the cache is bypassed, fetching image data errors
	// out, and the test fails, proving a cache hit skips the registry round trip entirely.
	f := &ivfuncs{
		Adapter:        types.DefaultTypeAdapter,
		policy:         pol,
		cosignVerifier: cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier: notary.NewVerifier(logr.Discard()),
		ivCache:        ivCache,
	}

	cacheRule := attestorCacheRule(signatureCacheRule, attestors)
	stored, err := ivCache.Set(context.TODO(), pol, cacheRule, image, true)
	assert.NoError(t, err)
	assert.True(t, stored)

	out2 := f.verify_image_signature_string_stringarray(f.NativeToValue(image), f.NativeToValue(attestors))
	assert.Equal(t, int64(len(attestors)), out2.Value())
}

func Test_impl_verify_image_signature_cache_miss_does_not_cache_failure(t *testing.T) {
	// a certificate that doesn't match the image's actual signer, so verification fails
	// while the image fetch itself still succeeds against the real registry.
	attestors := []v1beta1.Attestor{
		{
			Name: "notary",
			Notary: &v1beta1.Notary{
				Certs: &v1beta1.StringOrExpression{
					Value: "not-a-valid-certificate",
				},
			},
		},
	}
	pol := &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cache-policy-miss",
			UID:             "test-uid-miss",
			ResourceVersion: "1",
		},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors: attestors,
		},
	}
	// reuse the same signed image as the other tests: the fetch succeeds, but the bogus
	// cert above means verification never passes, so the count never reaches len(attestors)
	// and the result must not be cached.
	image := "ghcr.io/kyverno/test-verify-image:signed"

	imgCtx, err := imagedataloader.NewImageContext(nil)
	assert.NoError(t, err)

	ivCache, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(0),
		imageverifycache.WithTTLDuration(0),
	)
	assert.NoError(t, err)

	f := &ivfuncs{
		Adapter:        types.DefaultTypeAdapter,
		imgCtx:         imgCtx,
		policy:         pol,
		cosignVerifier: cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier: notary.NewVerifier(logr.Discard()),
		ivCache:        ivCache,
	}

	out := f.verify_image_signature_string_stringarray(f.NativeToValue(image), f.NativeToValue(attestors))
	count, ok := out.Value().(int64)
	assert.True(t, ok, "expected an integer result, got an error instead: %v", out.Value())
	assert.Less(t, count, int64(len(attestors)))

	cacheRule := attestorCacheRule(signatureCacheRule, attestors)
	found, err := ivCache.Get(context.TODO(), pol, cacheRule, image, true)
	assert.NoError(t, err)
	assert.False(t, found, "a partial or failed verification must never be cached")
}
