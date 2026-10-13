package evaluator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type proofImageContext struct {
	imagedataloader.ImageContext
	images []string
}

func (c *proofImageContext) Get(_ context.Context, image string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	c.images = append(c.images, image)
	return &imagedataloader.ImageData{}, nil
}

func TestRegistryEgressProofPerformsMetadataLookup(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "..", "test", "conformance", "chainsaw", "image-validating-policies", "standard", "registry-egress-hard-block")
	policyBytes, err := os.ReadFile(filepath.Join(fixture, "policy.yaml"))
	require.NoError(t, err)
	policy := &policiesv1beta1.ImageValidatingPolicy{}
	require.NoError(t, yaml.Unmarshal(policyBytes, policy))
	podBytes, err := os.ReadFile(filepath.Join(fixture, "unmatched-pod.yaml"))
	require.NoError(t, err)
	pod := &unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal(podBytes, &pod.Object))
	pod.SetLabels(map[string]string{"test.kyverno.io/registry-egress": "true"})

	images := &proofImageContext{}
	// No GVR-derived extractors: this proves the checked-in validation performs
	// its own lookup, rather than accidentally passing through image prefetch.
	compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
	require.Empty(t, errs)
	result, err := compiled.Evaluate(context.Background(), images, nil, pod.Object, nil, false, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Result, "valid local image metadata must satisfy the proof policy")
	require.Equal(t, []string{"registry.k8s.io/pause:3.10"}, images.images, "the proof must use the real image-loader interface, not a constant validation or fixture-only CEL metadata provider")
}
