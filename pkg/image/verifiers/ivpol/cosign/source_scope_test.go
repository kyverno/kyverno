package cosign

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
)

type sourceScopeTransport struct {
	target  string
	blocked bool
	calls   int
}

func (transport *sourceScopeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.calls++
	if transport.blocked {
		return nil, registryclient.ErrRegistryAddressBlocked
	}
	copy := request.Clone(request.Context())
	copy.URL.Host = transport.target
	copy.URL.Scheme = "http"
	return http.DefaultTransport.RoundTrip(copy)
}

func TestPolicySignatureRepositoryUsesNativeOptions(t *testing.T) {
	ctx := context.Background()
	var requests atomic.Int32
	registryHandler := registry.New()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		registryHandler.ServeHTTP(writer, request)
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	reference, err := name.ParseReference(host + "/app:latest")
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, empty.Image))
	transport := &sourceScopeTransport{target: host}
	loader, err := imagedataloader.New(nil, []remote.Option{remote.WithTransport(transport)}, []name.Option{name.Insecure})
	require.NoError(t, err)
	image, err := loader.FetchImageData(ctx, "8.8.8.8/app:latest", nil, []name.Option{name.Insecure})
	require.NoError(t, err)
	transport.blocked = true
	transport.calls = 0
	attestor := &v1beta1.Attestor{Cosign: &v1beta1.Cosign{
		Key:   &v1beta1.Key{Data: testPublicKey},
		CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true},
	}}
	verifier := NewVerifier(nil, logr.Discard())
	err = verifier.VerifyImageSignature(ctx, image, attestor)
	require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
	require.Positive(t, transport.calls)
	transport.calls = 0
	requests.Store(0)
	attestor.Cosign.Source = &v1beta1.Source{Repository: host + "/signatures"}
	err = verifier.VerifyImageSignature(ctx, image, attestor)
	require.Error(t, err)
	require.NotErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
	require.Positive(t, requests.Load(), "explicit repository must reach the native HTTP client")
	require.Zero(t, transport.calls, "resource digest must not be resolved again using native options")
}

func TestPolicyPublicKeyURLRemainsNative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(testPublicKey))
	}))
	defer server.Close()
	opts, err := checkOptions(context.Background(), &v1beta1.Cosign{
		Key:   &v1beta1.Key{KMS: server.URL},
		CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true},
	}, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, opts.SigVerifier)
}
