package internal

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An attestation without a type cannot be matched against statements, so it must fail the rule
// gracefully before any image is fetched.
func TestVerifyAttestationsWithoutType(t *testing.T) {
	t.Parallel()
	iv := &imageVerifier{
		logger: logr.Discard(),
		rule:   kyvernov1.Rule{Name: "verify-attestations"},
	}
	imageVerify := kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{{}},
	}

	resp, digest := iv.verifyAttestations(context.TODO(), imageVerify, apiutils.ImageInfo{})

	require.NotNil(t, resp)
	assert.Equal(t, engineapi.RuleStatusFail, resp.Status())
	assert.Contains(t, resp.Message(), "missing type")
	assert.Empty(t, digest)
}

func TestVerifyAttestationWithoutType(t *testing.T) {
	t.Parallel()
	iv := &imageVerifier{logger: logr.Discard()}

	err := iv.verifyAttestation([]map[string]any{{"type": "https://slsa.dev/provenance/v1"}}, kyvernov1.Attestation{}, apiutils.ImageInfo{})

	assert.ErrorContains(t, err, "a type is required")
}
