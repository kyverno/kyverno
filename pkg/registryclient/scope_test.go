package registryclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/config"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResourceImageExtractionEgress(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"audit", "enforce"} {
		for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
			for _, host := range []string{"127.0.0.1:5000", "169.254.169.254"} {
				t.Run(mode+"/"+field+"/"+host, func(t *testing.T) {
					t.Parallel()
					resource := unstructured.Unstructured{Object: map[string]interface{}{
						"apiVersion": "v1", "kind": "Pod",
						"metadata": map[string]interface{}{"name": "untrusted-image"},
						"spec": map[string]interface{}{
							field: []interface{}{map[string]interface{}{"name": "untrusted", "image": host + "/image:latest"}},
						},
					}}
					images, err := apiutils.ExtractImagesFromResource(resource, nil, config.NewDefaultConfiguration(false))
					require.NoError(t, err)
					require.Len(t, images[field], 1)
					credentials := &contextKeychain{}
					configured := New(WithAllowInsecureRegistry(true), WithEgressMode(mode), WithKeychain(credentials))
					options, names, err := configured.Options(context.Background())
					require.NoError(t, err)
					for _, image := range images[field] {
						reference, err := name.ParseReference(image.String(), names...)
						require.NoError(t, err)
						_, err = remote.Get(reference, options...)
						require.ErrorIs(t, err, ErrRegistryAddressBlocked)
					}
					require.Zero(t, credentials.calls)
				})
			}
		}
	}
}

func TestPolicyClientUsesNativeRegistryOptions(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"audit", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			handler := registry.New()
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				handler.ServeHTTP(response, request)
			}))
			defer server.Close()
			reference, err := name.ParseReference(strings.TrimPrefix(server.URL, "http://") + "/policy/image:latest")
			require.NoError(t, err)
			require.NoError(t, remote.Write(reference, empty.Image, remote.WithAuth(authn.Anonymous)))
			requests.Store(0)
			credentials := &contextKeychain{}
			configured := New(WithAllowInsecureRegistry(true), WithEgressMode(mode), WithKeychain(credentials)).(*client)
			resourceOptions, _, err := configured.Options(context.Background())
			require.NoError(t, err)
			_, err = remote.Get(reference, resourceOptions...)
			require.ErrorIs(t, err, ErrRegistryAddressBlocked)
			require.Zero(t, requests.Load())
			require.Zero(t, credentials.calls)
			policyOptions, _, err := configured.PolicyClient().Options(context.Background())
			require.NoError(t, err)
			_, err = remote.Get(reference, policyOptions...)
			require.NoError(t, err)
			require.Positive(t, requests.Load())
			require.Positive(t, credentials.calls)
			requests.Store(0)
			_, err = remote.Get(reference, resourceOptions...)
			require.ErrorIs(t, err, ErrRegistryAddressBlocked)
			require.Zero(t, requests.Load())
		})
	}
}
