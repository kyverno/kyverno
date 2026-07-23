//go:build integration && registry

// These tests drive the mutating phase against real container registries, so they perform the actual
// cosign and notary verification instead of reading a pre-stamped outcome. They are kept behind the
// extra "registry" build tag because they need outbound network (ghcr.io, and Rekor for keyless), and
// each one costs a registry round trip. Run them with:
//
//	go test -tags="integration registry" ./test/integration/ivpol/...
//
// Everything that can be asserted without a registry lives in handler_test.go and runs by default.

package ivpol_test

import (
	"context"
	"testing"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	ivpol "github.com/kyverno/kyverno/pkg/webhooks/resource/ivpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

const (
	// Images published by the kyverno test image repositories.
	signedImage   = "ghcr.io/kyverno/test-verify-image:signed"
	unsignedImage = "ghcr.io/kyverno/test-verify-image:unsigned"

	orgRegistry     = "ghcr.io/kyverno/test-images/cosign"
	keyedOrgImage   = orgRegistry + ":v3-traditional"
	keylessOrgImage = orgRegistry + ":v3-keyless"

	// GitHub Actions OIDC identity that signs the keyless org images.
	githubActionsIssuer = "https://token.actions.githubusercontent.com"
	githubWorkflowID    = "https://github.com/kyverno/test-images/.github/workflows/cosign.yml@refs/heads/main"
	rekorURL            = "https://rekor.sigstore.dev"
)

// orgCosignPubKey is the public key the kyverno test-images repository signs its key based images
// with (cosign/examples/cosign.pub). It differs from the key used for test-verify-image.
const orgCosignPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEPEDZl3iOJwr77T2bS9vgonwzERmG
PKd/xnmHKfvkbLquVC6NnH8dgPVq8p0H45H2H9CqzqGv+rn99xAWGLE30A==
-----END PUBLIC KEY-----`

// notaryCert is the certificate that signed ghcr.io/kyverno/test-verify-image:signed, the same one
// the in-tree ivpol engine test and the notary conformance policies use.
const notaryCert = `-----BEGIN CERTIFICATE-----
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

// applyOutcomePatch applies the JSON patch the mutating phase produced to the raw pod, the way the
// API server applies an admission patch before handing the object to the next webhook.
func applyOutcomePatch(t *testing.T, raw []byte, patch []byte) []byte {
	t.Helper()
	decoded, err := jsonpatch.DecodePatch(patch)
	require.NoError(t, err)
	patched, err := decoded.Apply(raw)
	require.NoError(t, err)
	return patched
}

// requireImageReachable skips the test when the registry cannot be reached, matching the convention
// the in-tree cosign verifier tests use so a developer without egress (or a transient registry
// outage) sees a skip instead of a spurious failure.
func requireImageReachable(t *testing.T, image string) {
	t.Helper()
	loader, err := imagedataloader.New(nil)
	require.NoError(t, err)
	if _, err := loader.FetchImageData(context.Background(), image); err != nil {
		t.Skipf("test image %s not accessible: %v", image, err)
	}
}

// cosignKeyedPolicy builds a policy that verifies images against a cosign public key. Key based
// signatures need no transparency log lookup, so this shape only depends on the registry.
func cosignKeyedPolicy(name, publicKey string) *policiesv1beta1.ImageValidatingPolicy {
	policy := newIvpol(name)
	policy.Spec.Attestors = []policiesv1beta1.Attestor{{
		Name: "cosign",
		Cosign: &policiesv1beta1.Cosign{
			Key:   &policiesv1beta1.Key{Data: publicKey},
			CTLog: &policiesv1beta1.CTLog{InsecureIgnoreTlog: true, InsecureIgnoreSCT: true},
		},
	}}
	return policy
}

// cosignKeylessPolicy builds a policy that verifies keyless signatures issued to a GitHub Actions
// workflow identity, which is how the kyverno test images are signed in CI.
func cosignKeylessPolicy(name, issuer, subject string) *policiesv1beta1.ImageValidatingPolicy {
	policy := newIvpol(name)
	policy.Spec.Attestors = []policiesv1beta1.Attestor{{
		Name: "cosign",
		Cosign: &policiesv1beta1.Cosign{
			Keyless: &policiesv1beta1.Keyless{
				Identities: []policiesv1beta1.Identity{{Issuer: issuer, Subject: subject}},
			},
			CTLog: &policiesv1beta1.CTLog{URL: rekorURL, InsecureIgnoreSCT: true},
		},
	}}
	return policy
}

// mutatePod runs the mutating webhook route for a pod carrying the given image and returns the
// admission response, which holds the verification outcome the phase recorded.
func mutatePod(t *testing.T, policyName, podName, namespace, image string) (bool, string) {
	t.Helper()
	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	raw := podRawWithImage(t, podName, namespace, image)
	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest(podName, namespace, raw), "", time.Now())
	return resp.Allowed, outcomeStatusFor(t, resp.Patch, policyName)
}

// TestMutate_CosignSignedImage_VerifiesSuccessfully is the end to end signature check: a real signed
// image is pulled from the registry, verified against the policy key, and recorded as passing.
func TestMutate_CosignSignedImage_VerifiesSuccessfully(t *testing.T) {
	requireImageReachable(t, signedImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("cosign-signed", cosignPubKey))
	waitForPolicyReady(t, "cosign-signed", "")

	allowed, status := mutatePod(t, "cosign-signed", "signed-pod", "default", signedImage)

	assert.True(t, allowed, "a correctly signed image must be admitted")
	assert.Equal(t, "pass", status, "a correctly signed image must record a passing verification")
}

// TestMutate_UnsignedImage_FailsVerification is the negative control for the check above: the same
// policy against an unsigned image records a failure.
func TestMutate_UnsignedImage_FailsVerification(t *testing.T) {
	requireImageReachable(t, unsignedImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("cosign-unsigned", cosignPubKey))
	waitForPolicyReady(t, "cosign-unsigned", "")

	_, status := mutatePod(t, "cosign-unsigned", "unsigned-pod", "default", unsignedImage)

	assert.Equal(t, "fail", status, "an unsigned image must record a failed verification")
}

// TestMutate_CosignKeyedOrgImage_VerifiesSuccessfully covers the key based images published by the
// kyverno test-images repository, which are signed with a different key than test-verify-image.
func TestMutate_CosignKeyedOrgImage_VerifiesSuccessfully(t *testing.T) {
	requireImageReachable(t, keyedOrgImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("cosign-org-keyed", orgCosignPubKey))
	waitForPolicyReady(t, "cosign-org-keyed", "")

	allowed, status := mutatePod(t, "cosign-org-keyed", "org-keyed-pod", "default", keyedOrgImage)

	assert.True(t, allowed, "an image signed with the org key must be admitted")
	assert.Equal(t, "pass", status, "an image signed with the org key must record a passing verification")
}

// TestMutate_CosignKeylessImage_VerifiesSuccessfully covers keyless (OIDC) signing, which also
// consults the Rekor transparency log.
func TestMutate_CosignKeylessImage_VerifiesSuccessfully(t *testing.T) {
	requireImageReachable(t, keylessOrgImage)

	createIvpolWithCleanup(t, cosignKeylessPolicy("cosign-keyless", githubActionsIssuer, githubWorkflowID))
	waitForPolicyReady(t, "cosign-keyless", "")

	allowed, status := mutatePod(t, "cosign-keyless", "keyless-pod", "default", keylessOrgImage)

	assert.True(t, allowed, "a keyless signed image from the expected workflow must be admitted")
	assert.Equal(t, "pass", status, "a keyless signed image must record a passing verification")
}

// TestMutate_KeylessWrongIdentity_FailsVerification proves the keyless identity is actually enforced:
// the image is signed, but by a different workflow than the policy trusts.
func TestMutate_KeylessWrongIdentity_FailsVerification(t *testing.T) {
	requireImageReachable(t, keylessOrgImage)

	policy := cosignKeylessPolicy("cosign-wrong-identity", githubActionsIssuer,
		"https://github.com/wrong/repo/.github/workflows/ci.yml@refs/heads/main")
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, "cosign-wrong-identity", "")

	_, status := mutatePod(t, "cosign-wrong-identity", "wrong-identity-pod", "default", keylessOrgImage)

	assert.NotEqual(t, "pass", status, "an image signed by another workflow identity must not pass")
}

// TestMutateThenValidate_TwoPhaseFlowAdmitsVerifiedPod wires both webhook phases together the way the
// API server does: the mutating phase verifies the image and stamps the outcome, and the validating
// phase reads that stamp and admits the pod. Neither phase is meaningful on its own.
func TestMutateThenValidate_TwoPhaseFlowAdmitsVerifiedPod(t *testing.T) {
	requireImageReachable(t, signedImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("two-phase", cosignPubKey))
	waitForPolicyReady(t, "two-phase", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), "two-phase")

	// Phase 1: verify the image and collect the outcomes the API server would apply to the object.
	raw := podRawWithImage(t, "two-phase-pod", "default", signedImage)
	mutateResp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("two-phase-pod", "default", raw), "", time.Now())
	require.True(t, mutateResp.Allowed, "the mutating phase must admit the pod")
	require.Equal(t, "pass", outcomeStatusFor(t, mutateResp.Patch, "two-phase"), "the mutating phase must verify the image")

	// Apply the patch, as the API server does, then run the validating phase on the patched pod.
	patched := applyOutcomePatch(t, raw, mutateResp.Patch)
	validateResp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("two-phase-pod", "default", patched), "", time.Now())

	assert.True(t, validateResp.Allowed, "the validating phase must admit a pod the mutating phase verified")
}

// TestMutate_NotarySignedImage_VerifiesSuccessfully covers the other supported signature format.
func TestMutate_NotarySignedImage_VerifiesSuccessfully(t *testing.T) {
	requireImageReachable(t, signedImage)

	policy := newIvpol("notary-signed")
	policy.Spec.Attestors = []policiesv1beta1.Attestor{{
		Name:   "notary",
		Notary: &policiesv1beta1.Notary{Certs: &policiesv1beta1.StringOrExpression{Value: notaryCert}},
	}}
	policy.Spec.Validations = []admissionregistrationv1.Validation{{
		Expression: "images.containers.map(image, verifyImageSignatures(image, [attestors.notary])).all(e, e > 0)",
		Message:    "failed to verify image with notary cert",
	}}
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, "notary-signed", "")

	allowed, status := mutatePod(t, "notary-signed", "notary-pod", "default", signedImage)

	assert.True(t, allowed, "a notary signed image must be admitted")
	assert.Equal(t, "pass", status, "a notary signed image must record a passing verification")
}
