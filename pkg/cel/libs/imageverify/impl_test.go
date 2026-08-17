package imageverify

import (
	"context"
	"encoding/json"
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
	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)

	options := []cel.EnvOption{
		cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
		Lib(nil, imgCtx, ivpol, nil, logr.Discard(), nil, NewImageVerificationResults()),
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
	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)

	options := []cel.EnvOption{
		cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
		Lib(nil, imgCtx, ivpol, nil, logr.Discard(), nil, NewImageVerificationResults()),
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

	cacheRule := attestorCacheRule(signatureCacheRule, "", attestors)
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

	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
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

	cacheRule := attestorCacheRule(signatureCacheRule, "", attestors)
	found, err := ivCache.Get(context.TODO(), pol, cacheRule, image, true)
	assert.NoError(t, err)
	assert.False(t, found, "a partial or failed verification must never be cached")
}

// Demonstrates that a verifyAttestationSignatures cache hit restores Cosign
// verifiedIntotoPayloads onto a fresh ImageData so extractPayload still works.
func Test_impl_verify_attestation_cache_hit_restores_payload(t *testing.T) {
	attestors := []v1beta1.Attestor{
		{
			Name: "github-keyless-attestation",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://token.actions.githubusercontent.com",
							Subject: "https://github.com/kyverno/test-images/.github/workflows/cosign.yml@refs/heads/main",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		},
	}
	attestations := []v1beta1.Attestation{
		{
			Name: "slsa",
			InToto: &v1beta1.InToto{
				Type: "https://slsa.dev/provenance/v1",
			},
		},
	}
	pol := &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "attestation-cache-policy",
			UID:             "test-uid-attestation-cache",
			ResourceVersion: "1",
		},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors:    attestors,
			Attestations: attestations,
		},
	}
	// Cosign/InToto image — Referrer/Notary GetPayload falls back to the registry and
	// would not reproduce the "intoto attestation payload cannot be fetch" error.
	image := "ghcr.io/kyverno/test-images/cosign:github-attestation"
	attestationName := "slsa"

	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)

	ivCache, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(0),
		imageverifycache.WithTTLDuration(0),
	)
	assert.NoError(t, err)

	f := &ivfuncs{
		Adapter:               types.DefaultTypeAdapter,
		imgCtx:                imgCtx,
		policy:                pol,
		attestationList:       attestationMap(pol),
		cosignVerifier:        cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:        notary.NewVerifier(logr.Discard()),
		ivCache:               ivCache,
		verifications:         NewImageVerificationResults(),
		pendingIntotoRestores: map[string]map[string][]byte{},
	}

	// 1. Cache miss: real Cosign verification populates verifiedIntotoPayloads and Sets the ivCache.
	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out), "cache-miss verification should not error: %v", out)
	assert.Equal(t, int64(len(attestors)), out.Value())

	// 2. extractPayload succeeds on the same ImageData that verification just populated.
	payload := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.False(t, types.IsError(payload), "extractPayload should succeed after a cache miss: %v", payload)
	missPayload := payload.Value()
	assert.NotNil(t, missPayload)

	// 3. New admission request: fresh imgCtx, same process-lifetime ivCache (already Set by step 1).
	imgCtx2, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)
	f.imgCtx = imgCtx2
	f.verifications = NewImageVerificationResults()

	// 4. Cache hit: restores verifiedIntotoPayloads onto the fresh ImageData from ivCache.
	out2 := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out2), "cache-hit verification should not error: %v", out2)
	assert.Equal(t, int64(len(attestors)), out2.Value())

	// 5. extractPayload after cache hit must match the miss-path payload exactly
	// (proves GetPayload→json.Marshal→AddVerifiedIntotoPayloads→GetPayload is lossless).
	payload2 := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.False(t, types.IsError(payload2), "extractPayload should succeed after attestation cache hit: %v", payload2)
	hitPayload := payload2.Value()
	assert.NotNil(t, hitPayload)
	assert.Equal(t, missPayload, hitPayload, "cache-restored payload must deeply equal the fresh-verification payload")
}

// Attestation verify-only (no extractPayload): miss then hit must both succeed with the
// SetWithPayload/GetWithPayload plumbing — payload caching must not break verify-only use.
func Test_impl_verify_attestation_cache_hit_without_extract_payload(t *testing.T) {
	attestors := []v1beta1.Attestor{
		{
			Name: "github-keyless-attestation",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://token.actions.githubusercontent.com",
							Subject: "https://github.com/kyverno/test-images/.github/workflows/cosign.yml@refs/heads/main",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		},
	}
	pol := &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "attestation-verify-only-policy",
			UID:             "test-uid-attestation-verify-only",
			ResourceVersion: "1",
		},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors: attestors,
			Attestations: []v1beta1.Attestation{
				{
					Name: "slsa",
					InToto: &v1beta1.InToto{
						Type: "https://slsa.dev/provenance/v1",
					},
				},
			},
		},
	}
	image := "ghcr.io/kyverno/test-images/cosign:github-attestation"
	attestationName := "slsa"

	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)

	ivCache, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(0),
		imageverifycache.WithTTLDuration(0),
	)
	assert.NoError(t, err)

	f := &ivfuncs{
		Adapter:               types.DefaultTypeAdapter,
		imgCtx:                imgCtx,
		policy:                pol,
		attestationList:       attestationMap(pol),
		cosignVerifier:        cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:        notary.NewVerifier(logr.Discard()),
		ivCache:               ivCache,
		verifications:         NewImageVerificationResults(),
		pendingIntotoRestores: map[string]map[string][]byte{},
	}

	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out), "cache-miss verification should not error: %v", out)
	assert.Equal(t, int64(len(attestors)), out.Value())

	imgCtx2, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)
	f.imgCtx = imgCtx2
	f.verifications = NewImageVerificationResults()

	out2 := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out2), "cache-hit verification should not error: %v", out2)
	assert.Equal(t, int64(len(attestors)), out2.Value())
}

// Two intoto attestation types on the same image: each cache entry is keyed by attestation
// name and stores a predicateType→payload map. After a fresh imgCtx, both cache hits must
// restore the correct payload with no cross-attestation leak/overwrite.
//
// This verifies cache-KEY isolation (generateKey/cacheRule differs per attestation name +
// attestor set), not within-map key collision: intotoPayloadsFromImage only ever returns a
// single-key map {attest.InToto.Type: bytes}, and production SetWithPayload callers never
// write a multi-key payload map today.
func Test_impl_verify_attestation_cache_hit_two_intoto_types_isolated(t *testing.T) {
	attestorsSlsa := []v1beta1.Attestor{
		{
			Name: "github-keyless-slsa",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://token.actions.githubusercontent.com",
							Subject: "https://github.com/kyverno/test-images/.github/workflows/cosign.yml@refs/heads/main",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		},
	}
	// Distinct attestor name so attestorCacheRule differs from the SLSA combo.
	attestorsCustom := []v1beta1.Attestor{
		{
			Name: "github-keyless-custom",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://token.actions.githubusercontent.com",
							Subject: "https://github.com/kyverno/test-images/.github/workflows/cosign.yml@refs/heads/main",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		},
	}
	const (
		slsaType   = "https://slsa.dev/provenance/v1"
		customType = "https://example.com/attestation/custom/v1"
		slsaName   = "slsa"
		customName = "custom"
	)
	pol := &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "attestation-two-types-policy",
			UID:             "test-uid-attestation-two-types",
			ResourceVersion: "1",
		},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors: append(append([]v1beta1.Attestor{}, attestorsSlsa...), attestorsCustom...),
			Attestations: []v1beta1.Attestation{
				{Name: slsaName, InToto: &v1beta1.InToto{Type: slsaType}},
				{Name: customName, InToto: &v1beta1.InToto{Type: customType}},
			},
		},
	}
	image := "ghcr.io/kyverno/test-images/cosign:github-attestation"

	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)

	ivCache, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(0),
		imageverifycache.WithTTLDuration(0),
	)
	assert.NoError(t, err)

	f := &ivfuncs{
		Adapter:               types.DefaultTypeAdapter,
		imgCtx:                imgCtx,
		policy:                pol,
		attestationList:       attestationMap(pol),
		cosignVerifier:        cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:        notary.NewVerifier(logr.Discard()),
		ivCache:               ivCache,
		verifications:         NewImageVerificationResults(),
		pendingIntotoRestores: map[string]map[string][]byte{},
	}

	// Miss path for SLSA: real Cosign verify + SetWithPayload.
	outSlsa := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(slsaName),
		f.NativeToValue(attestorsSlsa),
	)
	assert.False(t, types.IsError(outSlsa), "slsa cache-miss verification should not error: %v", outSlsa)
	assert.Equal(t, int64(len(attestorsSlsa)), outSlsa.Value())

	slsaMiss := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(slsaName))
	assert.False(t, types.IsError(slsaMiss), "slsa extractPayload after miss: %v", slsaMiss)
	slsaMissPayload := slsaMiss.Value()
	assert.NotNil(t, slsaMissPayload)

	// The public test image only carries SLSA provenance. Seed the second attestation's
	// cache entry (same key shape verify would write) so we can exercise two hit restores
	// without requiring a second signed predicate type on the image.
	customPayloadObj := map[string]any{
		"marker":        "custom-attestation-payload",
		"predicateType": customType,
	}
	customPayloadBytes, err := json.Marshal(customPayloadObj)
	assert.NoError(t, err)
	customRule := attestorCacheRule(attestationCacheRule, customName, attestorsCustom)
	stored, err := ivCache.SetWithPayload(context.TODO(), pol, customRule, image, true, map[string][]byte{
		customType: customPayloadBytes,
	})
	assert.NoError(t, err)
	assert.True(t, stored)

	// Fresh request: same ivCache, empty ImageData.
	imgCtx2, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)
	f.imgCtx = imgCtx2
	f.verifications = NewImageVerificationResults()

	outSlsaHit := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(slsaName),
		f.NativeToValue(attestorsSlsa),
	)
	assert.False(t, types.IsError(outSlsaHit), "slsa cache-hit verification should not error: %v", outSlsaHit)
	assert.Equal(t, int64(len(attestorsSlsa)), outSlsaHit.Value())

	outCustomHit := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(customName),
		f.NativeToValue(attestorsCustom),
	)
	assert.False(t, types.IsError(outCustomHit), "custom cache-hit verification should not error: %v", outCustomHit)
	assert.Equal(t, int64(len(attestorsCustom)), outCustomHit.Value())

	slsaHit := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(slsaName))
	assert.False(t, types.IsError(slsaHit), "slsa extractPayload after hit: %v", slsaHit)
	slsaHitPayload := slsaHit.Value()
	assert.Equal(t, slsaMissPayload, slsaHitPayload, "slsa restored payload must match miss-path payload")

	customHit := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(customName))
	assert.False(t, types.IsError(customHit), "custom extractPayload after hit: %v", customHit)
	customHitPayload := customHit.Value()
	assert.Equal(t, customPayloadObj, customHitPayload, "custom restored payload must match seeded payload")

	assert.NotEqual(t, slsaHitPayload, customHitPayload, "distinct attestation types must not share or overwrite payloads")
}

// Demonstrates that a degraded cache entry -- the cache reports "found" but
// carries no payload to restore, e.g. from a TTL-window entry predating the
// write-side fix that now skips caching on failed payload capture -- falls
// back to full re-verification instead of hard-failing the CEL expression.
// This is the fix requested in PR review for "cache-hit restore can deny
// valid admissions": missing/degraded payloads must be treated as a cache
// miss, not an error.
func Test_impl_verify_attestation_cache_hit_missing_payload_falls_back_to_reverify(t *testing.T) {
	attestors := []v1beta1.Attestor{
		{
			Name: "github-keyless-attestation",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://token.actions.githubusercontent.com",
							Subject: "https://github.com/kyverno/test-images/.github/workflows/cosign.yml@refs/heads/main",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		},
	}
	attestations := []v1beta1.Attestation{
		{
			Name: "slsa",
			InToto: &v1beta1.InToto{
				Type: "https://slsa.dev/provenance/v1",
			},
		},
	}
	pol := &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "attestation-degraded-cache-policy",
			UID:             "test-uid-attestation-degraded-cache",
			ResourceVersion: "1",
		},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors:    attestors,
			Attestations: attestations,
		},
	}
	image := "ghcr.io/kyverno/test-images/cosign:github-attestation"
	attestationName := "slsa"

	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)

	ivCache, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(0),
		imageverifycache.WithTTLDuration(0),
	)
	assert.NoError(t, err)

	f := &ivfuncs{
		Adapter:               types.DefaultTypeAdapter,
		imgCtx:                imgCtx,
		policy:                pol,
		attestationList:       attestationMap(pol),
		cosignVerifier:        cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:        notary.NewVerifier(logr.Discard()),
		ivCache:               ivCache,
		verifications:         NewImageVerificationResults(),
		pendingIntotoRestores: map[string]map[string][]byte{},
	}

	// Seed a degraded (presence-only) cache entry directly, simulating what a
	// failed payload capture used to leave behind before the write-side fix.
	// This exercises the READ-side fallback independent of the write-side skip.
	cacheRule := attestorCacheRule(attestationCacheRule, attestationName, attestors)
	stored, err := ivCache.SetWithPayload(context.TODO(), pol, cacheRule, image, true, nil)
	assert.NoError(t, err)
	assert.True(t, stored, "degraded presence-only entry must still be cacheable, matching pre-fix Set() behavior")

	// Cache reports found=true but there is nothing to restore. The fix must
	// NOT error the CEL expression here -- it must fall back to a real verify
	// and succeed, exactly as if this had been a genuine cache miss.
	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out), "degraded cache hit must fall back to re-verification, not error: %v", out)
	assert.Equal(t, int64(len(attestors)), out.Value())

	// extractPayload must succeed too: the fallback real-verify populates the
	// payload on this request's ImageData, same as a genuine cache miss would.
	payload := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.False(t, types.IsError(payload), "extractPayload should succeed after fallback re-verification: %v", payload)
	assert.NotNil(t, payload.Value())
}
