//go:build integration

package ivpol_test

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/test/integration/framework/imagetest"
	"github.com/stretchr/testify/assert"
)

func hermeticKeylessPolicy(name, issuer, subject, trustedRootJSON string) *policiesv1beta1.ImageValidatingPolicy {
	policy := newIvpol(name)
	policy.Spec.Credentials = &policiesv1beta1.Credentials{
		AllowInsecureRegistry: true,
	}
	policy.Spec.Attestors = []policiesv1beta1.Attestor{{
		Name: "cosign",
		Cosign: &policiesv1beta1.Cosign{
			Keyless: &policiesv1beta1.Keyless{
				Identities: []policiesv1beta1.Identity{{
					Issuer:  issuer,
					Subject: subject,
				}},
			},
			CTLog: &policiesv1beta1.CTLog{
				InsecureIgnoreTlog: true,
				InsecureIgnoreSCT:  true,
			},
			TrustedRoot: &policiesv1beta1.StringOrExpression{
				Value: trustedRootJSON,
			},
		},
	}}
	return policy
}

// TestValidate_HermeticKeyless_AdmitsSignedImage exercises full-pipeline Cosign keyless
// verification (KubeProvider -> CEL compiler -> engine -> validating webhook handler) against
// an in-process OCI 1.1 registry and virtual Sigstore instance with no external network access.
func TestValidate_HermeticKeyless_AdmitsSignedImage(t *testing.T) {
	reg := imagetest.NewRegistry(t)
	vs := imagetest.NewVirtualSigstore(t)

	subject := reg.PushSubject(t, "signed")
	bundle := vs.SignKeylessBundle(t, subject.Digest.Hex, imagetest.DefaultSubject, imagetest.DefaultIssuer, imagetest.DefaultPredicateType)
	reg.PushReferrer(t, subject, bundle, imagetest.BundleMediaType, imagetest.BundleMediaType)

	policy := hermeticKeylessPolicy("hermetic-keyless-admit", imagetest.DefaultIssuer, imagetest.DefaultSubject, vs.TrustedRootJSON(t))
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, policy.Name, "")

	allowed, warnings, message := validatePod(t, policy.Name, "signed-pod", "default", reg.PinnedRef("signed", subject))

	assert.True(t, allowed, "a keyless-signed image matching the policy identity and trusted root must be admitted, got denial: %s", message)
	assert.Empty(t, warnings, "a passing verification must not warn")
}

// TestValidate_HermeticKeyless_DeniesWrongIdentity verifies that the keyless issuer and subject
// constraints are enforced end-to-end when the signature itself is cryptographically valid.
func TestValidate_HermeticKeyless_DeniesWrongIdentity(t *testing.T) {
	reg := imagetest.NewRegistry(t)
	vs := imagetest.NewVirtualSigstore(t)

	subject := reg.PushSubject(t, "signed")
	bundle := vs.SignKeylessBundle(t, subject.Digest.Hex, imagetest.DefaultSubject, imagetest.DefaultIssuer, imagetest.DefaultPredicateType)
	reg.PushReferrer(t, subject, bundle, imagetest.BundleMediaType, imagetest.BundleMediaType)
	imageRef := reg.PinnedRef("signed", subject)
	trustedRoot := vs.TrustedRootJSON(t)

	tests := []struct {
		name    string
		issuer  string
		subject string
	}{
		{
			name:    "wrong-subject",
			issuer:  imagetest.DefaultIssuer,
			subject: "other-builder@example.com",
		},
		{
			name:    "wrong-issuer",
			issuer:  "https://other-issuer.example.com",
			subject: imagetest.DefaultSubject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyName := "hermetic-keyless-" + tt.name
			policy := hermeticKeylessPolicy(policyName, tt.issuer, tt.subject, trustedRoot)
			createIvpolWithCleanup(t, policy)
			waitForPolicyReady(t, policyName, "")

			allowed, _, message := validatePod(t, policyName, "wrong-identity-pod", "default", imageRef)

			assert.False(t, allowed, "an image signed by a non-matching identity must be denied")
			assert.Contains(t, message, signatureDenial, "the denial must come from the signature check")
		})
	}
}

// TestValidate_HermeticKeyless_DeniesUntrustedRoot verifies that a signature issued by a
// different Sigstore instance (either different TSA+CA root or different Fulcio CA with the same
// TSA) is rejected end-to-end.
func TestValidate_HermeticKeyless_DeniesUntrustedRoot(t *testing.T) {
	reg := imagetest.NewRegistry(t)
	signerVS := imagetest.NewVirtualSigstore(t)
	otherVS := imagetest.NewVirtualSigstore(t)

	subject := reg.PushSubject(t, "signed")
	bundle := signerVS.SignKeylessBundle(t, subject.Digest.Hex, imagetest.DefaultSubject, imagetest.DefaultIssuer, imagetest.DefaultPredicateType)
	reg.PushReferrer(t, subject, bundle, imagetest.BundleMediaType, imagetest.BundleMediaType)
	imageRef := reg.PinnedRef("signed", subject)

	tests := []struct {
		name        string
		trustedRoot string
	}{
		{
			name:        "different-instance",
			trustedRoot: otherVS.TrustedRootJSON(t),
		},
		{
			name:        "different-fulcio-ca",
			trustedRoot: imagetest.ComposeTrustedRootJSON(t, otherVS, signerVS),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyName := "hermetic-keyless-untrusted-" + tt.name
			policy := hermeticKeylessPolicy(policyName, imagetest.DefaultIssuer, imagetest.DefaultSubject, tt.trustedRoot)
			createIvpolWithCleanup(t, policy)
			waitForPolicyReady(t, policyName, "")

			allowed, _, message := validatePod(t, policyName, "untrusted-root-pod", "default", imageRef)

			assert.False(t, allowed, "an image signed by an untrusted Sigstore root must be denied")
			assert.Contains(t, message, signatureDenial, "the denial must come from the signature check")
		})
	}
}

// TestValidate_HermeticKeyless_DeniesUnsignedImage is the negative control: an unsigned image
// pushed to the same local registry must be denied.
func TestValidate_HermeticKeyless_DeniesUnsignedImage(t *testing.T) {
	reg := imagetest.NewRegistry(t)
	vs := imagetest.NewVirtualSigstore(t)

	subject := reg.PushSubject(t, "unsigned")

	policy := hermeticKeylessPolicy("hermetic-keyless-unsigned", imagetest.DefaultIssuer, imagetest.DefaultSubject, vs.TrustedRootJSON(t))
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, policy.Name, "")

	allowed, _, message := validatePod(t, policy.Name, "unsigned-pod", "default", reg.PinnedRef("unsigned", subject))

	assert.False(t, allowed, "an unsigned image must be denied")
	assert.Contains(t, message, signatureDenial, "the denial must come from the signature check")
}

// TestValidate_HermeticKeyless_LegacyV2Signature covers the non-bundle (.sig tag) keyless path
// end-to-end against the in-process registry and virtual Sigstore root, asserting admission with
// the issuing root and denial with a non-matching identity or an untrusted root.
func TestValidate_HermeticKeyless_LegacyV2Signature(t *testing.T) {
	reg := imagetest.NewRegistry(t)
	vs := imagetest.NewVirtualSigstore(t)
	otherVS := imagetest.NewVirtualSigstore(t)

	subject := reg.PushSubject(t, "legacy-signed")
	vs.SignLegacyKeyless(t, reg.Repo, "legacy-signed", imagetest.DefaultSubject, imagetest.DefaultIssuer)
	imageRef := reg.PinnedRef("legacy-signed", subject)

	t.Run("admits with trusted root", func(t *testing.T) {
		policy := hermeticKeylessPolicy("hermetic-keyless-legacy-admit", imagetest.DefaultIssuer, imagetest.DefaultSubject, vs.TrustedRootJSON(t))
		createIvpolWithCleanup(t, policy)
		waitForPolicyReady(t, policy.Name, "")

		allowed, warnings, message := validatePod(t, policy.Name, "legacy-signed-pod", "default", imageRef)

		assert.True(t, allowed, "a legacy v2 keyless-signed image must be admitted, got denial: %s", message)
		assert.Empty(t, warnings, "a passing verification must not warn")
	})

	t.Run("denies with wrong identity", func(t *testing.T) {
		policy := hermeticKeylessPolicy("hermetic-keyless-legacy-wrong-id", imagetest.DefaultIssuer, "other-builder@example.com", vs.TrustedRootJSON(t))
		createIvpolWithCleanup(t, policy)
		waitForPolicyReady(t, policy.Name, "")

		allowed, _, message := validatePod(t, policy.Name, "legacy-wrong-id-pod", "default", imageRef)

		assert.False(t, allowed, "a legacy v2 keyless-signed image with a non-matching subject must be denied")
		assert.Contains(t, message, signatureDenial, "the denial must come from the signature check")
	})

	t.Run("denies with untrusted root", func(t *testing.T) {
		policy := hermeticKeylessPolicy("hermetic-keyless-legacy-deny", imagetest.DefaultIssuer, imagetest.DefaultSubject, otherVS.TrustedRootJSON(t))
		createIvpolWithCleanup(t, policy)
		waitForPolicyReady(t, policy.Name, "")

		allowed, _, message := validatePod(t, policy.Name, "legacy-untrusted-pod", "default", imageRef)

		assert.False(t, allowed, "a legacy v2 keyless-signed image must be denied against an untrusted root")
		assert.Contains(t, message, signatureDenial, "the denial must come from the signature check")
	})
}
