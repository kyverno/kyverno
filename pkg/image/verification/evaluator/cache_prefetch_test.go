package evaluator

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	remote "github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type recordingImageContext struct {
	getCount int
	failGet  bool
}

func (c *recordingImageContext) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return nil
}

func (c *recordingImageContext) Get(ctx context.Context, image string, authOpts []remote.Option, nameOpts []name.Option) (*imagedataloader.ImageData, error) {
	c.getCount++
	if c.failGet {
		return nil, fmt.Errorf("registry unreachable: image %s fetch failed", image)
	}
	return &imagedataloader.ImageData{}, nil
}

func Test_Evaluate_CacheHitSkipsImageFetch(t *testing.T) {
	p := ivpol.DeepCopy()
	p.ObjectMeta = metav1.ObjectMeta{
		Name:            "cached-ivpol",
		UID:             "uid-1",
		ResourceVersion: "1",
	}
	p.Spec.ValidationConfigurations = policiesv1alpha1.ValidationConfiguration{
		VerifyDigest: ptr.To(false),
		Required:     ptr.To(false),
	}
	p.Spec.Validations = []admissionregistrationv1.Validation{
		{
			Expression: "images.bar.map(image, verifyImageSignatures(image, [attestors.notary])).all(e, e > 0)",
			Message:    "failed to verify image with notary cert",
		},
	}

	compiled, errList := NewCompiler(nil).Compile(p, nil)
	require.Empty(t, errList)

	cacheClient, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(100),
	)
	require.NoError(t, err)

	// 1. First evaluation: healthy image context to populate the cache
	healthyCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	require.NoError(t, err)
	verifications := imageverify.NewImageVerificationResults()

	res1, err := compiled.Evaluate(context.Background(), healthyCtx, cacheClient, verifications, nil, obj(signedImage), nil, false, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, res1)
	assert.True(t, res1.Result)

	// 2. Second evaluation (subsequent admission request):
	// Even if the registry is completely unreachable (failGet: true),
	// the image signature is cached, so Evaluate must NOT fetch from the registry.
	erroringCtx := &recordingImageContext{failGet: true}
	verifications2 := imageverify.NewImageVerificationResults()

	res2, err := compiled.Evaluate(context.Background(), erroringCtx, cacheClient, verifications2, nil, obj(signedImage), nil, false, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.True(t, res2.Result, "cached admission should pass even if registry is unreachable")
	assert.Equal(t, 0, erroringCtx.getCount, "cached image should not trigger any image fetch")

	// 3. Third evaluation: uncached image with erroringCtx MUST attempt to fetch and fail
	verifications3 := imageverify.NewImageVerificationResults()
	res3, err := compiled.Evaluate(context.Background(), erroringCtx, cacheClient, verifications3, nil, obj(unsignedImage), nil, false, nil, nil)
	assert.Error(t, err, "uncached image must fail to evaluate when registry is unreachable")
	assert.Nil(t, res3)
	assert.Greater(t, erroringCtx.getCount, 0, "uncached image must trigger an image fetch on cache miss")
}
