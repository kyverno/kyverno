//go:build integration && registry

// These tests drive the validating phase against real container registries, so they perform the
// actual cosign and notary verification. They are kept behind the extra "registry" build tag because
// they need outbound network (ghcr.io, and Rekor for keyless), and each one costs a registry round
// trip. Run them with:
//
//	go test -tags="integration registry" ./test/integration/ivpol/...
//
// Everything that can be asserted without a registry lives in handler_test.go and runs by default.

package ivpol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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
	// Images published by the kyverno test image repositories. These are real images the
	// registry-tagged tests pull and verify; the hermetic tests use an unresolvable reference instead
	// (see unverifiableImage in handler_test.go).
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

// requireImageReachable skips the test when the registry cannot be reached, matching the convention
// the in-tree cosign verifier tests use so a developer without egress (or a transient registry
// outage) sees a skip instead of a spurious failure.
func requireImageReachable(t *testing.T, image string) {
	t.Helper()
	loader, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	if _, err := loader.FetchImageData(context.Background(), image, nil, nil); err != nil {
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

// validatePod runs the validating webhook route for a pod carrying the given image and returns
// whether the request was admitted along with any warnings. Verification happens entirely in this
// phase (issue #16336): a signed image is admitted, an unsigned or untrusted one is denied.
func validatePod(t *testing.T, policyName, podName, namespace, image string) (bool, []string) {
	t.Helper()
	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	raw := podRawWithImage(t, podName, namespace, image)
	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest(podName, namespace, raw), "", time.Now())
	return resp.Allowed, resp.Warnings
}

// TestValidate_CosignSignedImage_Admits is the end to end signature check: a real signed image is
// pulled from the registry, verified against the policy key, and admitted.
func TestValidate_CosignSignedImage_Admits(t *testing.T) {
	requireImageReachable(t, signedImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("cosign-signed", cosignPubKey))
	waitForPolicyReady(t, "cosign-signed", "")

	allowed, warnings := validatePod(t, "cosign-signed", "signed-pod", "default", signedImage)

	assert.True(t, allowed, "a correctly signed image must be admitted")
	assert.Empty(t, warnings, "a passing verification must not warn")
}

// TestValidate_UnsignedImage_Denies is the negative control for the check above: the same policy
// against an unsigned image denies the pod.
func TestValidate_UnsignedImage_Denies(t *testing.T) {
	requireImageReachable(t, unsignedImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("cosign-unsigned", cosignPubKey))
	waitForPolicyReady(t, "cosign-unsigned", "")

	allowed, _ := validatePod(t, "cosign-unsigned", "unsigned-pod", "default", unsignedImage)

	assert.False(t, allowed, "an unsigned image must be denied")
}

// TestValidate_CosignKeyedOrgImage_Admits covers the key based images published by the kyverno
// test-images repository, which are signed with a different key than test-verify-image.
func TestValidate_CosignKeyedOrgImage_Admits(t *testing.T) {
	requireImageReachable(t, keyedOrgImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("cosign-org-keyed", orgCosignPubKey))
	waitForPolicyReady(t, "cosign-org-keyed", "")

	allowed, _ := validatePod(t, "cosign-org-keyed", "org-keyed-pod", "default", keyedOrgImage)

	assert.True(t, allowed, "an image signed with the org key must be admitted")
}

// TestValidate_CosignKeylessImage_Admits covers keyless (OIDC) signing, which also consults the Rekor
// transparency log.
func TestValidate_CosignKeylessImage_Admits(t *testing.T) {
	requireImageReachable(t, keylessOrgImage)

	createIvpolWithCleanup(t, cosignKeylessPolicy("cosign-keyless", githubActionsIssuer, githubWorkflowID))
	waitForPolicyReady(t, "cosign-keyless", "")

	allowed, _ := validatePod(t, "cosign-keyless", "keyless-pod", "default", keylessOrgImage)

	assert.True(t, allowed, "a keyless signed image from the expected workflow must be admitted")
}

// TestValidate_KeylessWrongIdentity_Denies proves the keyless identity is actually enforced: the
// image is signed, but by a different workflow than the policy trusts.
func TestValidate_KeylessWrongIdentity_Denies(t *testing.T) {
	requireImageReachable(t, keylessOrgImage)

	policy := cosignKeylessPolicy("cosign-wrong-identity", githubActionsIssuer,
		"https://github.com/wrong/repo/.github/workflows/ci.yml@refs/heads/main")
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, "cosign-wrong-identity", "")

	allowed, _ := validatePod(t, "cosign-wrong-identity", "wrong-identity-pod", "default", keylessOrgImage)

	assert.False(t, allowed, "an image signed by another workflow identity must be denied")
}

// TestValidate_SignedImage_AdmitsInSinglePhase confirms the post-#16336 single-phase model: the pod
// carries no pre-stamped outcome annotation, so the validating phase verifies the image on its own
// and admits it, with no mutating phase involved.
func TestValidate_SignedImage_AdmitsInSinglePhase(t *testing.T) {
	requireImageReachable(t, signedImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("single-phase", cosignPubKey))
	waitForPolicyReady(t, "single-phase", "")

	allowed, warnings := validatePod(t, "single-phase", "single-phase-pod", "default", signedImage)

	assert.True(t, allowed, "the validating phase must verify and admit a signed image on its own")
	assert.Empty(t, warnings, "a passing verification must not warn")
}

// TestValidate_NotarySignedImage_Admits covers the other supported signature format.
func TestValidate_NotarySignedImage_Admits(t *testing.T) {
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

	allowed, _ := validatePod(t, "notary-signed", "notary-pod", "default", signedImage)

	assert.True(t, allowed, "a notary signed image must be admitted")
}

// TestMutate_PinsDigestAndStampsNothingElse is the success path of digest pinning, and the reason
// issue #16808 was filed: mutateDigest defaults to true but the mutating handler used to be a stub,
// so tags were never pinned. A real image is resolved here and the route must emit exactly one
// replace patch, targeting the container image field and carrying the resolved digest.
//
// The patch is also asserted to be the only one, which is the issue #16336 invariant on the path
// where mutation actually succeeds: the route must not additionally stamp an image verification
// outcome annotation for the validating route to trust.
func TestMutate_PinsDigestAndStampsNothingElse(t *testing.T) {
	requireImageReachable(t, signedImage)

	createIvpolWithCleanup(t, cosignKeyedPolicy("mutate-pin-digest", cosignPubKey))
	waitForPolicyReady(t, "mutate-pin-digest", "")

	resp := mutatePod(t, "mutate-pin-digest", "pinned-pod", "default", signedImage)
	require.True(t, resp.Allowed, "a resolvable image must be admitted by the mutating route")
	require.NotEmpty(t, resp.Patch, "mutateDigest is enabled, so the tag must be pinned (issue #16808)")

	var patches []map[string]any
	require.NoError(t, json.Unmarshal(resp.Patch, &patches))
	require.Len(t, patches, 1, "digest pinning must be the only mutation the route performs")

	assert.Equal(t, "replace", patches[0]["op"])
	assert.Equal(t, "/spec/containers/0/image", patches[0]["path"],
		"the patch must target the image field, not an annotation (issue #16336)")

	value, ok := patches[0]["value"].(string)
	require.True(t, ok, "the patched image must be a string")
	assert.True(t, strings.HasPrefix(value, signedImage+"@sha256:"),
		"the image must keep its original reference and gain a digest, got %q", value)
}

// TestMutate_PinsResolvableImageDespiteUnresolvableOne proves digest pinning is per image. A pod
// with one resolvable and one unresolvable image still gets the resolvable one pinned: abandoning
// every patch because a single image could not be resolved would leave images unpinned that the
// policy was perfectly able to pin. ClusterPolicy resolves each image independently for the same
// reason. The policy only warns here, so the failure does not block and the partial patch is
// observable in the response.
func TestMutate_PinsResolvableImageDespiteUnresolvableOne(t *testing.T) {
	requireImageReachable(t, signedImage)

	policy := cosignKeyedPolicy("mutate-partial-pin", cosignPubKey)
	policy.Spec.ValidationAction = []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn}
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, "mutate-partial-pin", "")

	raw := podRawWithImages(t, "partial-pod", "default", signedImage, unverifiableImage)
	resp := mutatePodRaw(t, "mutate-partial-pin", "partial-pod", "default", raw)

	require.True(t, resp.Allowed, "a warn only policy must not block on a digest pinning failure")
	assert.NotEmpty(t, resp.Warnings, "the image that could not be resolved must still be reported")

	var patches []map[string]any
	require.NoError(t, json.Unmarshal(resp.Patch, &patches))
	require.Len(t, patches, 1, "the resolvable image must still be pinned")

	assert.Equal(t, "/spec/containers/0/image", patches[0]["path"],
		"only the resolvable container must be patched")
	value, ok := patches[0]["value"].(string)
	require.True(t, ok, "the patched image must be a string")
	assert.True(t, strings.HasPrefix(value, signedImage+"@sha256:"),
		"the resolvable image must gain a digest, got %q", value)
}
