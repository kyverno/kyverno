package internal

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestEvaluateRuleValidation(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	res := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "test-pod", "namespace": "default"},
		},
	}
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}
	policyCtx, err := policycontext.NewPolicyContext(jp, res, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyCtx = policyCtx.WithPolicy(policy)

	// 1. Rule with no validation returns nil
	ivNoValidation := &imageVerifier{
		logger:        logr.Discard(),
		policyContext: policyCtx,
		rule:          kyvernov1.Rule{Name: "test-rule"},
	}
	assert.NoError(t, ivNoValidation.evaluateRuleValidation(context.Background()))

	// 2. Rule with validation where Deny condition matches returns error
	ivEntryFail := kyvernov1.ImageVerification{
		Validation: kyvernov1.ValidateImageVerification{
			Deny: &kyvernov1.Deny{},
		},
	}
	ivEntryFail.Validation.Deny.SetAnyAllConditions(kyvernov1.AnyAllConditions{
		AllConditions: []kyvernov1.Condition{
			{
				RawKey:   &apiextv1.JSON{Raw: []byte(`"true"`)},
				Operator: kyvernov1.ConditionOperators["Equals"],
				RawValue: &apiextv1.JSON{Raw: []byte(`"true"`)},
			},
		},
	})
	ivFail := &imageVerifier{
		logger:        logr.Discard(),
		policyContext: policyCtx,
		rule: kyvernov1.Rule{
			Name:         "test-rule",
			VerifyImages: []kyvernov1.ImageVerification{ivEntryFail},
		},
	}
	err = ivFail.evaluateRuleValidation(context.Background())
	assert.Error(t, err)

	// 3. Rule with validation where Deny condition does not match returns nil
	ivEntryPass := kyvernov1.ImageVerification{
		Validation: kyvernov1.ValidateImageVerification{
			Deny: &kyvernov1.Deny{},
		},
	}
	ivEntryPass.Validation.Deny.SetAnyAllConditions(kyvernov1.AnyAllConditions{
		AllConditions: []kyvernov1.Condition{
			{
				RawKey:   &apiextv1.JSON{Raw: []byte(`"true"`)},
				Operator: kyvernov1.ConditionOperators["Equals"],
				RawValue: &apiextv1.JSON{Raw: []byte(`"false"`)},
			},
		},
	})
	ivPass := &imageVerifier{
		logger:        logr.Discard(),
		policyContext: policyCtx,
		rule: kyvernov1.Rule{
			Name:         "test-rule",
			VerifyImages: []kyvernov1.ImageVerification{ivEntryPass},
		},
	}
	assert.NoError(t, ivPass.evaluateRuleValidation(context.Background()))
}
