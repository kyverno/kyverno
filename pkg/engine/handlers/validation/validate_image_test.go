package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// TestValidateImageHandler_RequiredEnforcedWithNonMatchingImage ensures a non-matching image is skipped rather than aborting evaluation, so the matching image still produces a rule response.
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
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, true)
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
	}
}

const verifyImageAttestorPolicy = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "verify-attestor"
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
				"attestors": [{
					"entries": [{
						"keys": {
							"publicKeys": "dummy"
						}
					}]
				}]
			}]
		}]
	}
}`

const priorStageMetadataPod = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod",
		"namespace": "default",
		"annotations": {
			"kyverno.io/verify-images-scoped": "{\"version\":1,\"policies\":{\"/verify-attestor\":{\"verify-image\":{\"ghcr.io/verified/app:v1\":\"pass\"}}}}"
		}
	},
	"spec": {
		"containers": [{
			"name": "app",
			"image": "ghcr.io/verified/app:v1"
		}]
	}
}`

const verifyDigestOnlyPolicy = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "verify-digest-only"
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
				"verifyDigest": true
			}]
		}]
	}
}`

const unpinnedImagePod = `{
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
		}]
	}
}`

// erroringRegistryClientFactory simulates a registry credential/client
// resolution failure. Used to prove that a stored, pass-claiming annotation
// is never consulted when the validating webhook can't independently verify.
type erroringRegistryClientFactory struct{}

func (erroringRegistryClientFactory) GetClient(context.Context, *kyvernov1.ImageRegistryCredentials, string, []string) (engineapi.RegistryClient, error) {
	return nil, fmt.Errorf("registry unreachable in test")
}

// TestValidateImage_DoesNotTrustPriorStageMetadata proves the validating
// handler performs its own verification regardless of pre-existing
// verification metadata already present on the resource: with no reachable
// verifier, the rule must not report a pass, no matter what that metadata
// claims.
func TestValidateImage_DoesNotTrustPriorStageMetadata(t *testing.T) {
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageAttestorPolicy), &cpol))

	resource, err := kubeutils.BytesToUnstructured([]byte(priorStageMetadataPod))
	require.NoError(t, err)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)

	rule := cpol.Spec.Rules[0]
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, erroringRegistryClientFactory{}, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	h := handler.(validateImageHandler)
	logger := logr.Discard()

	_, responses := h.Process(context.Background(), logger, policyContext, *resource, rule, nil, nil)
	require.NotEmpty(t, responses, "expected a rule response")
	assert.Equal(t, engineapi.RuleStatusError, responses[0].Status(), "verification must be attempted, and its infrastructure failure reported, regardless of pre-existing verification metadata")
	assert.NotContains(t, responses[0].Message(), "pass", "pre-existing verification metadata must never be echoed back as a pass")
}

// TestValidateImage_NilRegistryClientFactoryErrors pins that a missing
// registry dependency is reported as a rule error: the factory is an optional
// engine argument, and verification here needs registry access.
func TestValidateImage_NilRegistryClientFactoryErrors(t *testing.T) {
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageAttestorPolicy), &cpol))

	resource, err := kubeutils.BytesToUnstructured([]byte(priorStageMetadataPod))
	require.NoError(t, err)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)

	rule := cpol.Spec.Rules[0]
	for name, factory := range map[string]engineapi.RegistryClientFactory{
		"nil factory":              nil,
		"factory without a client": nilClientRegistryClientFactory{},
	} {
		t.Run(name, func(t *testing.T) {
			handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, factory, nil, nil, true)
			require.NoError(t, err)
			require.NotNil(t, handler)

			h := handler.(validateImageHandler)
			_, responses := h.Process(context.Background(), logr.Discard(), policyContext, *resource, rule, nil, nil)
			require.NotEmpty(t, responses, "expected a rule response")
			assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
			assert.Contains(t, responses[0].Message(), "no registry client configured")
		})
	}
}

// TestValidateImage_SkipReferenceNeedsNoRegistryClient pins that a reference
// the rule skips is honoured without acquiring a registry client. verifyImage
// matches skipImageReferences internally, so reaching that decision used to
// require a client first and reported an error when none was available.
func TestValidateImage_SkipReferenceNeedsNoRegistryClient(t *testing.T) {
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageAttestorPolicy), &cpol))
	cpol.Spec.Rules[0].VerifyImages[0].SkipImageReferences = []string{"*"}

	resource, err := kubeutils.BytesToUnstructured([]byte(priorStageMetadataPod))
	require.NoError(t, err)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)

	rule := cpol.Spec.Rules[0]
	// A nil factory would be an error for any image that must be verified, so
	// a Skip here proves no client was needed.
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	h := handler.(validateImageHandler)
	_, responses := h.Process(context.Background(), logr.Discard(), policyContext, *resource, rule, nil, nil)
	require.NotEmpty(t, responses, "expected a rule response")
	assert.NotEqual(t, engineapi.RuleStatusError, responses[0].Status(), "a skipped reference must not need a registry client: %s", responses[0].Message())
}

// nilClientRegistryClientFactory models a factory built with no global client:
// it returns that client unchanged, so the caller gets no error and no client.
type nilClientRegistryClientFactory struct{}

func (nilClientRegistryClientFactory) GetClient(context.Context, *kyvernov1.ImageRegistryCredentials, string, []string) (engineapi.RegistryClient, error) {
	return nil, nil
}

// TestVerifyDigestFailsClosedBeforeSkipMapping pins the ordering the design
// requires: a verifyDigest rule against an unpinned image fails closed, and
// this check runs before "no attestors/attestations configured" is mapped to
// Skip. Collapsing this ordering would silently exempt verifyDigest-only
// rules from enforcement.
func TestVerifyDigestFailsClosedBeforeSkipMapping(t *testing.T) {
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyDigestOnlyPolicy), &cpol))

	resource, err := kubeutils.BytesToUnstructured([]byte(unpinnedImagePod))
	require.NoError(t, err)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)

	rule := cpol.Spec.Rules[0]
	// No rclientFactory/ivCache: if the verifyDigest check did not run first,
	// this handler would need a registry client to reach the "no
	// attestors/attestations" branch and would panic or error for the wrong
	// reason instead of failing on the missing digest.
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	h := handler.(validateImageHandler)
	logger := logr.Discard()

	_, responses := h.Process(context.Background(), logger, policyContext, *resource, rule, nil, nil)
	require.NotEmpty(t, responses, "expected a rule response")
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "missing digest")
}

const verifyImageVariableReferencePolicy = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "verify-variable-reference"
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
			"verifyImages": [
				{
					"imageReferences": ["ghcr.io/verified/*"],
					"required": false
				},
				{
					"imageReferences": ["{{ overridePattern }}"],
					"required": true,
					"attestors": [{
						"entries": [{
							"keys": {
								"publicKeys": "dummy"
							}
						}]
					}]
				}
			]
		}]
	}
}`

// TestValidateImage_SubstitutesVariablesBeforeMatchingAndVerifying mirrors the
// mutating path's variable resolution: a rule field expressed as a variable
// (here, a second imageReferences entry) must be resolved before it is used
// to match images or to verify them. If substitution is skipped, the second
// entry's unresolved placeholder never matches any image, that entry is
// silently skipped, and only the first (unconditional Skip) entry's result is
// reported. With substitution wired up, the second entry's real attestor
// requirement is reached and its verification-infrastructure failure is
// reported as Error -- proof it was actually evaluated.
func TestValidateImage_SubstitutesVariablesBeforeMatchingAndVerifying(t *testing.T) {
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageVariableReferencePolicy), &cpol))

	resource, err := kubeutils.BytesToUnstructured([]byte(unpinnedImagePod))
	require.NoError(t, err)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)
	require.NoError(t, policyContext.JSONContext().AddVariable("overridePattern", "ghcr.io/verified/*"))

	rule := cpol.Spec.Rules[0]
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, erroringRegistryClientFactory{}, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	h := handler.(validateImageHandler)
	logger := logr.Discard()

	_, responses := h.Process(context.Background(), logger, policyContext, *resource, rule, nil, nil)
	require.NotEmpty(t, responses, "expected a rule response")
	assert.Equal(t, engineapi.RuleStatusError, responses[0].Status(), responses[0].Message())
}

const verifyImageVariablePolicy = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "verify-variable"
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
				"imageReferences": ["{{ request.object.metadata.annotations.allowedImage }}"],
				"required": true,
				"verifyDigest": false
			}]
		}]
	}
}`

const variableContainerPod = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod",
		"namespace": "default",
		"annotations": {
			"allowedImage": "ghcr.io/verified/*"
		}
	},
	"spec": {
		"containers": [{
			"name": "app",
			"image": "ghcr.io/verified/app:v1"
		}]
	}
}`

func TestValidateImageHandler_ImageReferencesWithVariable(t *testing.T) {
	t.Parallel()
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageVariablePolicy), &cpol))
	resource, err := kubeutils.BytesToUnstructured([]byte(variableContainerPod))
	require.NoError(t, err)
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)
	rule := cpol.Spec.Rules[0]
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)
	logger := logr.Discard()
	_, responses := handler.Process(context.Background(), logger, policyContext, *resource, rule, nil, nil)
	require.NotEmpty(t, responses)
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Equal(t, engineapi.ImageVerify, responses[0].RuleType())
	assert.Contains(t, responses[0].Message(), "rule requires image verification")
}

func TestValidateImageHandler_MutatedResourceImageReferencesWithVariable(t *testing.T) {
	t.Parallel()
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageVariablePolicy), &cpol))

	initialPodJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default",
			"annotations": {
				"allowedImage": "ghcr.io/verified/*"
			}
		},
		"spec": {
			"containers": [{
				"name": "app",
				"image": "docker.io/unmutated/old:v1"
			}]
		}
	}`
	initialResource, err := kubeutils.BytesToUnstructured([]byte(initialPodJSON))
	require.NoError(t, err)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *initialResource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*initialResource)

	rule := cpol.Spec.Rules[0]
	handler, err := NewValidateImageHandler(policyContext, *initialResource, rule, cfg, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	// Simulate preceding rule mutation updating container image to the verified image
	mutatedResource, err := kubeutils.BytesToUnstructured([]byte(variableContainerPod))
	require.NoError(t, err)
	require.NoError(t, policyContext.JSONContext().AddResource(mutatedResource.Object))

	logger := logr.Discard()
	_, responses := handler.Process(context.Background(), logger, policyContext, *mutatedResource, rule, nil, nil)
	require.NotEmpty(t, responses)
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Equal(t, engineapi.ImageVerify, responses[0].RuleType())
	assert.Contains(t, responses[0].Message(), "rule requires image verification")
}

func TestValidateImageHandler_ImageReferencesWithPerImageMessage(t *testing.T) {
	t.Parallel()
	policyWithMsg := strings.Replace(
		verifyImageVariablePolicy,
		`"verifyDigest": false`,
		`"verifyDigest": false, "validate": {"message": "{{ image.reference }} is not signed"}`,
		1,
	)
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(policyWithMsg), &cpol))
	resource, err := kubeutils.BytesToUnstructured([]byte(variableContainerPod))
	require.NoError(t, err)
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)
	rule := cpol.Spec.Rules[0]
	handler, err := NewValidateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)
	logger := logr.Discard()
	_, responses := handler.Process(context.Background(), logger, policyContext, *resource, rule, nil, nil)
	require.NotEmpty(t, responses)
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Equal(t, engineapi.ImageVerify, responses[0].RuleType())
	assert.Contains(t, responses[0].Message(), "rule requires image verification")
}
