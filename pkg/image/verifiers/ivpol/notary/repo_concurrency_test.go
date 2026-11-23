package notary

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	ggcrtypes "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Referrers are verified in parallel, so the repository, and the ImageData
// cache behind it, must be safe to call concurrently. Run with -race.
func TestFetchSignatureBlobConcurrently(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)
	repo, err := name.NewRepository(strings.TrimPrefix(srv.URL, "http://") + "/app")
	require.NoError(t, err)
	subject, err := random.Image(64, 1)
	require.NoError(t, err)
	subjectDesc, err := partial.Descriptor(subject)
	require.NoError(t, err)
	require.NoError(t, remote.Write(repo.Digest(subjectDesc.Digest.String()), subject))

	var descs []string
	for i := range 8 {
		blob, err := mutate.AppendLayers(empty.Image, static.NewLayer(fmt.Appendf(nil, `{"n":%d}`, i), ggcrtypes.MediaType("application/json")))
		require.NoError(t, err)
		digest, err := blob.Digest()
		require.NoError(t, err)
		require.NoError(t, remote.Write(repo.Digest(digest.String()), blob))
		descs = append(descs, digest.String())
	}

	idf, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	img, err := idf.FetchImageData(context.Background(), repo.Digest(subjectDesc.Digest.String()).String(), nil, nil)
	require.NoError(t, err)
	client := NewRepository(img)

	var wg sync.WaitGroup
	for _, d := range descs {
		wg.Go(func() {
			desc, err := client.Resolve(context.Background(), d)
			if assert.NoError(t, err) {
				_, _, err = client.FetchSignatureBlob(context.Background(), desc)
				assert.NoError(t, err)
			}
		})
	}
	wg.Wait()
}
