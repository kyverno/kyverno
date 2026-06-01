package internal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testGoodDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000001"
	testEvilDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000002"
)

type digestRegistryClient struct {
	digest string
	err    error
}

func (c *digestRegistryClient) ForRef(context.Context, string) (*engineapi.ImageData, error) {
	return &engineapi.ImageData{}, nil
}

func (c *digestRegistryClient) FetchImageDescriptor(context.Context, string) (*gcrremote.Descriptor, error) {
	if c.err != nil {
		return nil, c.err
	}
	hash, err := v1.NewHash(c.digest)
	if err != nil {
		return nil, err
	}
	return &gcrremote.Descriptor{
		Descriptor: v1.Descriptor{
			Digest:    hash,
			MediaType: types.DockerManifestSchema2,
		},
	}, nil
}

func (c *digestRegistryClient) Keychain() authn.Keychain {
	return authn.DefaultKeychain
}

func (c *digestRegistryClient) Options(context.Context) ([]gcrremote.Option, error) {
	return nil, nil
}

func (c *digestRegistryClient) NameOptions() []name.Option {
	return nil
}

func testImageInfo(digest string) apiutils.ImageInfo {
	return apiutils.ImageInfo{
		ImageInfo: imageutils.ImageInfo{
			Registry:         "ghcr.io",
			Path:             "acme/app",
			Tag:              "signed",
			Reference:        "ghcr.io/acme/app",
			ReferenceWithTag: "ghcr.io/acme/app:signed",
			Digest:           digest,
		},
		Pointer: "/spec/containers/0/image",
	}
}

func TestVerifyCacheDigestBinding_MatchingDigest(t *testing.T) {
	t.Parallel()

	iv := &imageVerifier{
		rclient: &digestRegistryClient{digest: testGoodDigest},
	}
	imageInfo := testImageInfo("")

	ok, digest, err := iv.verifyCacheDigestBinding(context.Background(), imageInfo.String(), imageInfo, testGoodDigest)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, testGoodDigest, digest)
}

func TestVerifyCacheDigestBinding_MovedTagReVerifies(t *testing.T) {
	t.Parallel()

	iv := &imageVerifier{
		rclient: &digestRegistryClient{digest: testEvilDigest},
	}
	imageInfo := testImageInfo("")

	ok, digest, err := iv.verifyCacheDigestBinding(context.Background(), imageInfo.String(), imageInfo, testGoodDigest)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, digest)
}

func TestVerifyCacheDigestBinding_PinnedDigestMismatch(t *testing.T) {
	t.Parallel()

	iv := &imageVerifier{
		rclient: &digestRegistryClient{digest: testGoodDigest},
	}
	imageInfo := testImageInfo(testEvilDigest)

	ok, digest, err := iv.verifyCacheDigestBinding(context.Background(), imageInfo.String(), imageInfo, testGoodDigest)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, digest)
}

// TestVerifyImagesCacheControl_MovedTagDoesNotPinEvilDigest mirrors the reporter control:
// a warmed cache for a tag must not admit and pin a different digest when the tag moved.
func TestVerifyImagesCacheControl_MovedTagDoesNotPinEvilDigest(t *testing.T) {
	t.Parallel()

	const imageRef = "ghcr.io/acme/app:signed"

	cacheClient, err := imageverifycache.New(
		imageverifycache.WithCacheEnableFlag(true),
		imageverifycache.WithMaxSize(1000),
		imageverifycache.WithTTLDuration(time.Hour),
	)
	require.NoError(t, err)

	var cpol kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "cache-control",
			"uid": "11111111-1111-1111-1111-111111111111",
			"resourceVersion": "42"
		},
		"spec": {
			"rules": [{
				"name": "verify"
			}]
		}
	}`), &cpol))

	podJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "test", "namespace": "default"},
		"spec": {
			"containers": [{
				"name": "c",
				"image": "ghcr.io/acme/app:signed"
			}]
		}
	}`
	resource, err := kubeutils.BytesToUnstructured([]byte(podJSON))
	require.NoError(t, err)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := policycontext.NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resource)

	rule := kyvernov1.Rule{
		Name: "verify",
		VerifyImages: []kyvernov1.ImageVerification{
			{
				ImageReferences: []string{"ghcr.io/acme/*"},
				MutateDigest:    true,
				UseCache:        true,
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									PublicKeys: testAttestorPublicKey,
								},
							},
						},
					},
				},
			},
		},
	}

	set, err := cacheClient.Set(context.Background(), &cpol, rule.Name, imageRef, testGoodDigest, true)
	require.NoError(t, err)
	require.True(t, set)

	ivm := &engineapi.ImageVerificationMetadata{}
	iv := NewImageVerifier(
		logr.Discard(),
		&digestRegistryClient{digest: testEvilDigest},
		cacheClient,
		policyContext,
		rule,
		ivm,
	)

	imageVerify := *rule.VerifyImages[0].Convert()
	patches, responses := iv.Verify(context.Background(), imageVerify, []apiutils.ImageInfo{testImageInfo("")}, cfg)

	for _, resp := range responses {
		if resp.Message() != "verified from cache" {
			continue
		}
		for _, patch := range patches {
			value, ok := patch.Value.(string)
			if !ok {
				continue
			}
			if strings.Contains(value, testEvilDigest) {
				t.Fatalf("cache hit pinned unverified digest %s via mutateDigest", testEvilDigest)
			}
		}
	}
}

const testAttestorPublicKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEMKLYTatU9CUsrA5Td6jXiZTolwsx
HZKwYP5XkHhU436FGDD5Zi2nVFem6AbzXWHssIQRkAI3yJgKkB4J6Qe4OQ==
-----END PUBLIC KEY-----`
