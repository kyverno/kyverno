package evaluator

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func policyWithLedger(required *bool, recorded map[string]bool) *compiledPolicy {
	ledger := imageverify.NewVerificationLedger()
	for image, ok := range recorded {
		ledger.Record(image, ok)
	}
	return &compiledPolicy{
		validationConfig: policiesv1alpha1.ValidationConfiguration{Required: required},
		ledger:           ledger,
	}
}

// The whole point of required: a policy can pass all of its CEL expressions
// without any image having been cryptographically checked, for instance when the
// expression is a constant or tolerates a zero verification count.
func TestEnforceRequired_DeniesImageThatWasNeverChecked(t *testing.T) {
	t.Parallel()
	c := policyWithLedger(nil, nil)

	err := c.enforceRequired([]string{"ghcr.io/kyverno/test-verify-image:signed"})

	require.Error(t, err, "required defaults to true, so an unchecked image must be rejected")
	assert.Contains(t, err.Error(), "ghcr.io/kyverno/test-verify-image:signed")
	assert.Contains(t, err.Error(), "no signature or attestation check")
}

func TestEnforceRequired_DeniesImageThatFailedVerification(t *testing.T) {
	t.Parallel()
	image := "ghcr.io/kyverno/test-verify-image:unsigned"
	c := policyWithLedger(nil, map[string]bool{image: false})

	err := c.enforceRequired([]string{image})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed signature or attestation verification",
		"an image that was checked and rejected must report differently from one never checked")
}

func TestEnforceRequired_AllowsVerifiedImage(t *testing.T) {
	t.Parallel()
	image := "ghcr.io/kyverno/test-verify-image:signed"
	c := policyWithLedger(nil, map[string]bool{image: true})

	assert.NoError(t, c.enforceRequired([]string{image}))
}

// Opting out has to be honoured, otherwise existing policies that legitimately
// only inspect image metadata would break.
func TestEnforceRequired_DisabledSkipsTheCheck(t *testing.T) {
	t.Parallel()
	c := policyWithLedger(ptr.To(false), nil)

	assert.NoError(t, c.enforceRequired([]string{"ghcr.io/kyverno/test-verify-image:signed"}))
}

// Every matched image must be verified, not just one of them, so a pod that
// smuggles in a second unverified image is still rejected.
func TestEnforceRequired_DeniesWhenOnlySomeImagesAreVerified(t *testing.T) {
	t.Parallel()
	verified := "ghcr.io/kyverno/test-verify-image:signed"
	unverified := "ghcr.io/kyverno/other:latest"
	c := policyWithLedger(nil, map[string]bool{verified: true})

	err := c.enforceRequired([]string{verified, unverified})

	require.Error(t, err)
	assert.Contains(t, err.Error(), unverified)
	assert.NotContains(t, err.Error(), verified)
}

// images is already filtered by matchImageReferences, so a policy that matched
// nothing has nothing to require and must not deny.
func TestEnforceRequired_NoMatchedImagesPasses(t *testing.T) {
	t.Parallel()
	c := policyWithLedger(nil, nil)

	assert.NoError(t, c.enforceRequired(nil))
}
