package validation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const verifyImageRequiredPolicy = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "verify-required"
	},
	"spec": {
		"rules": [{
			"name": "verify-image",
			"match": {
				"any": [{
					"resources": {
						"kinds": ["Pod"]
					}
				}]
			},
			"verifyImages": [{
				"imageReferences": ["ghcr.io/verified/*"],
				"required": true,
				"verifyDigest": false
			}]
		}]
	}
}`

const multiContainerPod = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod",
		"namespace": "default"
	},
	"spec": {
		"initContainers": [{
			"name": "sidecar",
			"image": "docker.io/busybox:1.36"
		}],
		"containers": [{
			"name": "app",
			"image": "ghcr.io/verified/app:v1"
		}]
	}
}`

// TestValidateImageHandler_RequiredEnforcedWithNonMatchingImage ensures verifyImages(required)
// still fails unverified matching images when another container uses a non-matching image.
// Non-matching images must be skipped, not abort evaluation with an empty response.
func TestValidateImageHandler_RequiredEnforcedWithNonMatchingImage(t *testing.T) {
	t.Parallel()

	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageRequiredPolicy), &cpol))

	resource, err := kubeutils.BytesToUnstructured([]byte(multiContainerPod))
	require.NoError(t, err)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)

	rule := cpol.Spec.Rules[0]
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	h := handler.(validateImageHandler)
	logger := logr.Discard()

	// Map iteration order is non-deterministic; repeat to cover early-return on non-matching image.
	for i := 0; i < 32; i++ {
		_, responses := h.Process(context.Background(), logger, policyContext, *resource, rule, nil, nil)
		require.NotEmpty(t, responses, "iteration %d: expected a rule response, got none (verifyImages bypass)", i)
		assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status(), "iteration %d", i)
		assert.Equal(t, engineapi.ImageVerify, responses[0].RuleType(), "iteration %d", i)
		assert.Contains(t, responses[0].Message(), "unverified image", "iteration %d", i)
	}
}

const verifyImageTwoMatchingPolicy = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "verify-two-images"
	},
	"spec": {
		"rules": [{
			"name": "verify-image",
			"match": {
				"any": [{
					"resources": {
						"kinds": ["Pod"]
					}
				}]
			},
			"verifyImages": [{
				"imageReferences": ["ghcr.io/verified/*"],
				"required": true,
				"verifyDigest": false
			}]
		}]
	}
}`

const twoMatchingContainersPod = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod",
		"namespace": "default"
	},
	"spec": {
		"containers": [{
			"name": "app",
			"image": "ghcr.io/verified/app:v1"
		}, {
			"name": "sidecar",
			"image": "ghcr.io/verified/sidecar:v1"
		}]
	}
}`

// TestValidateImageHandler_FailureReportsCorrectImages ensures that when multiple
// images match a verifyImages rule and fail verification, all failing image names
// are reported -- not an arbitrary one picked by non-deterministic map iteration.
func TestValidateImageHandler_FailureReportsCorrectImages(t *testing.T) {
	t.Parallel()
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageTwoMatchingPolicy), &cpol))
	resource, err := kubeutils.BytesToUnstructured([]byte(twoMatchingContainersPod))
	require.NoError(t, err)
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)
	rule := cpol.Spec.Rules[0]
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)
	h := handler.(validateImageHandler)
	logger := logr.Discard()
	// Run 32 iterations to catch non-deterministic map iteration -- before the fix
	// the reported image varied across runs; after the fix both images must always appear.
	for i := 0; i < 32; i++ {
		_, responses := h.Process(context.Background(), logger, policyContext, *resource, rule, nil, nil)
		require.NotEmpty(t, responses, "iteration %d: expected a rule response", i)
		assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status(), "iteration %d", i)
		msg := responses[0].Message()
		assert.Contains(t, msg, "ghcr.io/verified/app:v1", "iteration %d: app image must appear in failure message", i)
		assert.Contains(t, msg, "ghcr.io/verified/sidecar:v1", "iteration %d: sidecar image must appear in failure message", i)
	}
}
