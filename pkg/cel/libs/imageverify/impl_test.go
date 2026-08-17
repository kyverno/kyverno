package imageverify

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/cosign"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/notary"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// payloadCacheMaxSize is large enough for a real Cosign/Notary attestation
// payload. WithMaxSize(0) uses the production default of 1000, which is sized
// for presence-only signature entries and will silently reject a multi-KB
// payload (see pkg/image/verification/cache/client_test.go).
const payloadCacheMaxSize int64 = 1_000_000

// stubImageContext implements imagedataloader.ImageContext and always errors.
// Referrer degrade/wrong-key tests use it so a trusted cache hit would succeed
// without touching a registry, while a real fallback to re-verify errors.
type stubImageContext struct {
	err error
}

func (s stubImageContext) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return s.err
}

func (s stubImageContext) Get(context.Context, string, []remote.Option, []name.Option) (*imagedataloader.ImageData, error) {
	return nil, s.err
}

func newTestIVCache(t *testing.T, maxSize int64) imageverifycache.Client {
	t.Helper()
	ivCache, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(maxSize),
		imageverifycache.WithTTLDuration(0),
	)
	assert.NoError(t, err)
	return ivCache
}

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

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

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

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

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

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter:                    types.DefaultTypeAdapter,
		imgCtx:                     imgCtx,
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
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

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter:                    types.DefaultTypeAdapter,
		imgCtx:                     imgCtx,
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
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
// attestor set), not within-map key collision: attestationPayloadFromImage only ever returns
// a single-key map {attest.InToto.Type: bytes}, and production SetWithPayload callers never
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

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter:                    types.DefaultTypeAdapter,
		imgCtx:                     imgCtx,
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
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

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter:                    types.DefaultTypeAdapter,
		imgCtx:                     imgCtx,
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
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

// referrerTestFixture builds the Attestor/Attestation/policy fixture shared by
// the Referrer/Notary cache tests below: a Notary attestor and an OCI-referrer
// "sbom" attestation, matching the real signed test image.
func referrerTestFixture(policyName, uid string) (attestors []v1beta1.Attestor, attestationName string, pol *v1beta1.ImageValidatingPolicy) {
	attestors = []v1beta1.Attestor{
		{
			Name: "notary",
			Notary: &v1beta1.Notary{
				Certs: &v1beta1.StringOrExpression{
					Value: cert,
				},
			},
		},
	}
	attestations := []v1beta1.Attestation{
		{
			Name: "sbom",
			Referrer: &v1beta1.Referrer{
				Type: "sbom/cyclone-dx",
			},
		},
	}
	pol = &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            policyName,
			UID:             k8stypes.UID(uid),
			ResourceVersion: "1",
		},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors:    attestors,
			Attestations: attestations,
		},
	}
	return attestors, "sbom", pol
}

// Demonstrates that a verifyAttestationSignatures cache hit restores a
// Referrer/Notary payload onto a fresh ImageData so extractPayload still
// works -- the OCI-referrer counterpart to
// Test_impl_verify_attestation_cache_hit_restores_payload, covering the
// attestation type that previously had no restore path at all (#17130).
func Test_impl_verify_referrer_attestation_cache_hit_restores_payload(t *testing.T) {
	attestors, attestationName, pol := referrerTestFixture("referrer-attestation-cache-policy", "test-uid-referrer-attestation-cache")
	image := "ghcr.io/kyverno/test-verify-image:signed"

	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	assert.NoError(t, err)

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter:                    types.DefaultTypeAdapter,
		imgCtx:                     imgCtx,
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
	}

	// 1. Cache miss: real Notary verification populates verifiedReferrers and Sets the ivCache.
	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out), "cache-miss verification should not error: %v", out)
	assert.Equal(t, int64(len(attestors)), out.Value())

	cacheRule := attestorCacheRule(attestationCacheRule, attestationName, attestors)
	found, cached, err := ivCache.GetWithPayload(context.TODO(), pol, cacheRule, image, true)
	assert.NoError(t, err)
	assert.True(t, found, "successful verification must write a cache entry")
	assert.Contains(t, cached, "sbom/cyclone-dx", "cache write must store the Referrer payload under the artifact type")
	assert.NotEmpty(t, cached["sbom/cyclone-dx"])

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

	// 4. Cache hit: restores the cached payload without touching img.verifiedReferrers.
	out2 := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out2), "cache-hit verification should not error: %v", out2)
	assert.Equal(t, int64(len(attestors)), out2.Value())

	// 5. extractPayload after cache hit must match the miss-path payload exactly,
	// returned straight from the cache without ever calling GetPayload's
	// Referrer fallback (which would otherwise fetch unverified data).
	payload2 := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.False(t, types.IsError(payload2), "extractPayload should succeed after referrer attestation cache hit: %v", payload2)
	hitPayload := payload2.Value()
	assert.NotNil(t, hitPayload)
	assert.Equal(t, missPayload, hitPayload, "cache-restored payload must deeply equal the fresh-verification payload")
}

// This is the regression test for #17130 ("extractPayload() silently returns
// unverified data on cache hit for Referrer/Notary attestations"): before the
// fix, the cache-hit degrade check only applied to InToto attestations, so a
// degraded (presence-only) Referrer/Notary cache entry was blindly trusted as
// a valid hit instead of triggering re-verification. Both verify and
// extractPayload would still appear to succeed in that case -- the actual bug
// was that verification never really ran, and extractPayload's data came from
// GetPayload's unverified live-registry fallback rather than a cryptographic
// check.
//
// imgCtx is a stub that always errors: a trusted degraded hit would return
// success without calling Get, so this test would fail. Falling back to
// re-verify must call Get and surface the stub error. This stays network-free
// (the live miss→hit path is Test_impl_verify_referrer_attestation_cache_hit_restores_payload).
func Test_impl_verify_referrer_attestation_cache_hit_missing_payload_falls_back_to_reverify(t *testing.T) {
	attestors, attestationName, pol := referrerTestFixture("referrer-attestation-degraded-cache-policy", "test-uid-referrer-attestation-degraded-cache")
	image := "ghcr.io/kyverno/test-verify-image:signed"
	registryErr := errors.New("registry fetch disabled")

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter:                    types.DefaultTypeAdapter,
		imgCtx:                     stubImageContext{err: registryErr},
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
	}

	// Seed a degraded (presence-only) cache entry directly, simulating what a
	// failed payload capture -- or a pre-fix write -- used to leave behind for
	// a Referrer/Notary attestation.
	cacheRule := attestorCacheRule(attestationCacheRule, attestationName, attestors)
	stored, err := ivCache.SetWithPayload(context.TODO(), pol, cacheRule, image, true, nil)
	assert.NoError(t, err)
	assert.True(t, stored, "degraded presence-only entry must still be cacheable")

	// Cache reports found=true but there is nothing to restore. The fix must
	// NOT trust this as a valid hit -- it must fall back to re-verification,
	// which errors because the stub ImageContext refuses a registry fetch.
	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.True(t, types.IsError(out), "degraded referrer cache hit must not be trusted as success; got: %v", out)

	// extractPayload must also fail closed rather than the SDK's unverified
	// live-fetch fallback. The stub makes any Get() fail, so a pass here
	// would mean we served unverified data (or trusted the empty cache).
	payload := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.True(t, types.IsError(payload), "extractPayload must not return unverified data on a degraded Referrer cache entry: %v", payload)
}

// Regression test for #17130: proves a Referrer/Notary cache hit is served
// entirely from the cache, never via a live registry fetch. imgCtx is left
// nil on purpose -- if either verify or extractPayload touched the registry
// at all, this test would panic instead of silently passing by coincidence.
// Comparing hit-path vs miss-path payloads alone (as
// Test_impl_verify_referrer_attestation_cache_hit_restores_payload does)
// can't tell a genuine cache-serve apart from an unverified fallback fetch
// that happens to return the same real bytes; seeding a marker payload that
// does not exist on the real image makes the distinction unambiguous.
func Test_impl_verify_referrer_attestation_cache_hit_serves_cached_payload_not_live_fetch(t *testing.T) {
	attestors, attestationName, pol := referrerTestFixture("referrer-attestation-marker-policy", "test-uid-referrer-attestation-marker")
	image := "ghcr.io/kyverno/test-verify-image:signed"

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter: types.DefaultTypeAdapter,
		// imgCtx is left nil on purpose, see comment above.
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
	}

	markerPayload := map[string]any{"marker": "referrer-cache-serve-proof", "bomFormat": "CycloneDX"}
	markerBytes, err := json.Marshal(markerPayload)
	assert.NoError(t, err)

	cacheRule := attestorCacheRule(attestationCacheRule, attestationName, attestors)
	stored, err := ivCache.SetWithPayload(context.TODO(), pol, cacheRule, image, true, map[string][]byte{
		"sbom/cyclone-dx": markerBytes,
	})
	assert.NoError(t, err)
	assert.True(t, stored)

	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out), "cache hit must succeed without touching the (nil) registry client: %v", out)
	assert.Equal(t, int64(len(attestors)), out.Value())

	payload := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.False(t, types.IsError(payload), "extractPayload must succeed without touching the (nil) registry client: %v", payload)
	assert.Equal(t, markerPayload, payload.Value(), "extractPayload must return the cached marker, not a live registry fetch")
}

// Regression test for the "one-shot then fail-open" variant of #17130: a
// policy commonly calls extractPayload() more than once for the same
// image+attestation (e.g. checking several payload fields across separate
// CEL expressions). The first extractPayload() call after a Referrer/Notary
// cache hit must not be the only one on the safe path -- every subsequent
// call for the same image+attestation must keep returning the cached,
// verified payload too, never fall through to the SDK's unverified
// live-fetch fallback. imgCtx is left nil on purpose: if any call touched
// the registry, this test would panic instead of passing by coincidence.
func Test_impl_verify_referrer_attestation_cache_hit_repeated_extract_payload_stays_verified(t *testing.T) {
	attestors, attestationName, pol := referrerTestFixture("referrer-attestation-repeat-policy", "test-uid-referrer-attestation-repeat")
	image := "ghcr.io/kyverno/test-verify-image:signed"

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter:                    types.DefaultTypeAdapter,
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
	}

	markerPayload := map[string]any{"marker": "repeat-extract-proof"}
	markerBytes, err := json.Marshal(markerPayload)
	assert.NoError(t, err)

	cacheRule := attestorCacheRule(attestationCacheRule, attestationName, attestors)
	stored, err := ivCache.SetWithPayload(context.TODO(), pol, cacheRule, image, true, map[string][]byte{
		"sbom/cyclone-dx": markerBytes,
	})
	assert.NoError(t, err)
	assert.True(t, stored)

	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.False(t, types.IsError(out), "cache hit should not error: %v", out)

	// First extractPayload() call.
	first := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.False(t, types.IsError(first), "first extractPayload call should not error: %v", first)
	assert.Equal(t, markerPayload, first.Value())

	// Second extractPayload() call for the SAME image+attestation, simulating
	// a policy checking a second field in a separate CEL expression. Before
	// the fix, the first call consumed and deleted the pending restore, so
	// this second call fell through to GetPayload()'s unverified fallback.
	second := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.False(t, types.IsError(second), "second extractPayload call should not error: %v", second)
	assert.Equal(t, markerPayload, second.Value(), "second extractPayload call must still return the verified cached payload, not fall open")
}

// Regression test for a cache entry whose payload map is non-empty but
// missing the specific artifact type this attestation expects (e.g. a stale
// entry left behind under the same cache rule name). A naive
// len(payloads)==0 check would treat this as a trustworthy hit; the fix
// requires the SPECIFIC expected key to be present, so this must also fall
// back to full re-verification rather than being trusted or silently
// falling through to an unverified live fetch.
//
// Same stub-ImageContext proof as the degraded-entry test: trusting the
// wrong-key hit would succeed without Get(); falling back must error.
func Test_impl_verify_referrer_attestation_cache_hit_wrong_key_falls_back_to_reverify(t *testing.T) {
	attestors, attestationName, pol := referrerTestFixture("referrer-attestation-wrongkey-policy", "test-uid-referrer-attestation-wrongkey")
	image := "ghcr.io/kyverno/test-verify-image:signed"
	registryErr := errors.New("registry fetch disabled")

	ivCache := newTestIVCache(t, payloadCacheMaxSize)

	f := &ivfuncs{
		Adapter:                    types.DefaultTypeAdapter,
		imgCtx:                     stubImageContext{err: registryErr},
		policy:                     pol,
		attestationList:            attestationMap(pol),
		cosignVerifier:             cosign.NewVerifier(nil, logr.Discard()),
		notaryVerifier:             notary.NewVerifier(logr.Discard()),
		ivCache:                    ivCache,
		verifications:              NewImageVerificationResults(),
		pendingAttestationRestores: map[string]map[string][]byte{},
	}

	// Seed a non-empty payload map, but under the WRONG artifact type key --
	// not the "sbom/cyclone-dx" this attestation expects.
	cacheRule := attestorCacheRule(attestationCacheRule, attestationName, attestors)
	stored, err := ivCache.SetWithPayload(context.TODO(), pol, cacheRule, image, true, map[string][]byte{
		"some/other-artifact-type": []byte(`{"unexpected":"data"}`),
	})
	assert.NoError(t, err)
	assert.True(t, stored)

	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue(image),
		f.NativeToValue(attestationName),
		f.NativeToValue(attestors),
	)
	assert.True(t, types.IsError(out), "wrong-key cache hit must not be trusted as success; got: %v", out)

	payload := f.payload_string_string(f.NativeToValue(image), f.NativeToValue(attestationName))
	assert.True(t, types.IsError(payload), "extractPayload must not return unverified data for a wrong-key Referrer cache entry: %v", payload)
}
