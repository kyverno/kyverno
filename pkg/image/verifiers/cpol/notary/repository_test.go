package notary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	notationregistry "github.com/notaryproject/notation-go/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

var (
	imageRef = "ghcr.io/kyverno/test-verify-image:signed"
	ctx      = context.Background()
)

func TestResolve(t *testing.T) {
	nameRef, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := remote.Head(nameRef)
	assert.NilError(t, err)

	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)

	repositoryClient := newRepository(nil, ref)

	desc, err := repositoryClient.Resolve(ctx, repoDesc.Digest.String())
	assert.NilError(t, err)
	assert.Equal(t, desc.Digest.String(), "sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
	assert.Equal(t, desc.MediaType, "application/vnd.docker.distribution.manifest.v2+json")
}

func TestListSignatures(t *testing.T) {
	nameRef, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := remote.Head(nameRef)
	assert.NilError(t, err)

	ociDesc := v1ToOciSpecDescriptor(*repoDesc)
	assert.Equal(t, ociDesc.Digest.String(), repoDesc.Digest.String())

	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)

	repositoryClient := newRepository(nil, ref)
	fn := func(_ []ocispec.Descriptor) error {
		return nil
	}

	err = repositoryClient.ListSignatures(ctx, ociDesc, fn)
	assert.NilError(t, err)
}

func TestFetchSignatureBlob(t *testing.T) {
	nameRef, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := remote.Head(nameRef)
	assert.NilError(t, err)

	ociDesc := v1ToOciSpecDescriptor(*repoDesc)
	assert.Equal(t, ociDesc.Digest.String(), repoDesc.Digest.String())

	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)

	repositoryClient := newRepository(nil, ref)

	referrers, err := remote.Referrers(ref.Context().Digest(ociDesc.Digest.String()))
	assert.NilError(t, err)
	referrersDescs, err := referrers.IndexManifest()
	assert.NilError(t, err)

	for _, d := range referrersDescs.Manifests {
		if d.ArtifactType == notationregistry.ArtifactTypeNotation {
			_, desc, err := repositoryClient.FetchSignatureBlob(ctx, v1ToOciSpecDescriptor(d))
			assert.NilError(t, err)
			assert.Equal(t, desc.MediaType, "application/jose+json")
		}
	}
}

// TestRepositoryClientHonoursCallContext asserts every registry round trip made
// on notation's behalf is bound to the context notation passes into the
// Repository method, not to whichever context happened to build the client's
// remote options. Without that binding the admission webhook's deadline never
// reaches the registry, so a registry that tarpits the connection holds a
// webhook worker for as long as it likes.
func TestRepositoryClientHonoursCallContext(t *testing.T) {
	var contacted atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		contacted.Store(true)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// httptest listens on 127.0.0.1, which go-containerregistry resolves over
	// plain http, so no TLS plumbing is needed here.
	ref, err := name.ParseReference(strings.TrimPrefix(srv.URL, "http://") + "/test/image:signed")
	assert.NilError(t, err)

	const dgst = "sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105"
	desc := ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    digest.Digest(dgst),
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	client := newRepository(nil, ref)
	calls := map[string]func(context.Context) error{
		"Resolve": func(ctx context.Context) error {
			_, err := client.Resolve(ctx, dgst)
			return err
		},
		"ListSignatures": func(ctx context.Context) error {
			return client.ListSignatures(ctx, desc, func([]ocispec.Descriptor) error { return nil })
		},
		"FetchSignatureBlob": func(ctx context.Context) error {
			_, _, err := client.FetchSignatureBlob(ctx, desc)
			return err
		},
	}

	for label, call := range calls {
		t.Run(label, func(t *testing.T) {
			contacted.Store(false)
			err := call(cancelled)
			assert.ErrorContains(t, err, context.Canceled.Error())
			assert.Assert(t, !contacted.Load(), "registry was contacted with an already cancelled context")
		})
	}
}
