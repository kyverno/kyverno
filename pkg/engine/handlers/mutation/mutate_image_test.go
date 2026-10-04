package mutation

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

const verifyImageVariableMutatePolicy = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "verify-variable-mutate"
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

const variableMutatePod = `{
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

func TestNewMutateImageHandler_StaticNoMatch(t *testing.T) {
	resource, err := kubeutils.BytesToUnstructured([]byte(variableMutatePod))
	require.NoError(t, err)
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)

	rule := kyvernov1.Rule{
		VerifyImages: []kyvernov1.ImageVerification{
			{
				ImageReferences: []string{"ghcr.io/unmatched/*"},
			},
		},
	}
	handler, err := NewMutateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, nil, true)
	require.NoError(t, err)
	assert.Nil(t, handler)
}

func TestNewMutateImageHandler_WithVariable(t *testing.T) {
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageVariableMutatePolicy), &cpol))
	resource, err := kubeutils.BytesToUnstructured([]byte(variableMutatePod))
	require.NoError(t, err)
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)
	rule := cpol.Spec.Rules[0]

	handler, err := NewMutateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, nil, true)
	require.NoError(t, err)
	assert.NotNil(t, handler)
}

func TestMutateImageHandler_Process_NoMatchingImagesAfterSubst(t *testing.T) {
	podUnmatchedJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default",
			"annotations": {
				"allowedImage": "ghcr.io/non-matching/*"
			}
		},
		"spec": {
			"containers": [{
				"name": "app",
				"image": "ghcr.io/verified/app:v1"
			}]
		}
	}`
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(verifyImageVariableMutatePolicy), &cpol))
	resource, err := kubeutils.BytesToUnstructured([]byte(podUnmatchedJSON))
	require.NoError(t, err)
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)
	rule := cpol.Spec.Rules[0]

	handler, err := NewMutateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	res, responses := handler.Process(context.Background(), logr.Discard(), policyContext, *resource, rule, nil, nil)
	assert.Empty(t, responses)
	assert.Equal(t, *resource, res)
}

func TestMutateImageHandler_Process_VariableSubstError(t *testing.T) {
	policyWithBadVar := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "test"},
		"spec": {
			"rules": [{
				"name": "rule1",
				"verifyImages": [{
					"imageReferences": ["{{ unresolvable_variable }}"],
					"required": true
				}]
			}]
		}
	}`
	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(policyWithBadVar), &cpol))
	resource, err := kubeutils.BytesToUnstructured([]byte(variableMutatePod))
	require.NoError(t, err)
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)
	rule := cpol.Spec.Rules[0]

	handler, err := NewMutateImageHandler(policyContext, *resource, rule, cfg, nil, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	_, responses := handler.Process(context.Background(), logr.Discard(), policyContext, *resource, rule, nil, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
	assert.Equal(t, engineapi.ImageVerify, responses[0].RuleType())
}
