package libs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
)

func TestImageDataContextDistinguishesResourceAndPolicyImages(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	handler := registry.New()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		handler.ServeHTTP(writer, request)
	}))
	defer server.Close()
	policyImage := strings.TrimPrefix(server.URL, "http://") + "/policy/image:latest"
	policyReference, err := name.ParseReference(policyImage)
	require.NoError(t, err)
	require.NoError(t, remote.Write(policyReference, empty.Image))
	workloadImage := strings.TrimPrefix(server.URL, "http://") + "/workload/image:latest"
	fetcher, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	provider := &contextProvider{imagedata: fetcher}
	for _, category := range []string{"containers", "initContainers", "ephemeralContainers"} {
		for _, location := range []string{"pod", "template", "cronjob"} {
			t.Run(category+"/"+location, func(t *testing.T) {
				spec := map[string]any{category: []any{map[string]any{"image": workloadImage}}}
				object := map[string]any{"spec": spec}
				switch location {
				case "template":
					object = map[string]any{"spec": map[string]any{"template": object}}
				case "cronjob":
					object = map[string]any{"spec": map[string]any{"jobTemplate": map[string]any{"spec": map[string]any{"template": object}}}}
				}
				images := ImageDataContext(context.Background(), provider, object)
				requests.Store(0)
				_, err := images.GetImageData(workloadImage, nil)
				require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
				require.Zero(t, requests.Load())
				_, err = images.GetImageData(policyImage, nil)
				require.NoError(t, err, "author-selected image retains native access even on the same registry")
				require.Positive(t, requests.Load())
			})
		}
	}
}

func TestImageDataContextGuardsOldResourceImages(t *testing.T) {
	t.Parallel()
	object := map[string]any{"spec": map[string]any{"ephemeralContainers": []any{map[string]any{"image": "169.254.169.254/workload:latest"}}}}
	_, err := ImageDataContext(context.Background(), NewFakeContextProvider(), nil, object).GetImageData("169.254.169.254/workload:latest", nil)
	require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
}
